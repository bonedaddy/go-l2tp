package l2tp

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// slowStartState represents state for the transport sequence numbers
// and slow start/congestion avoidance algorithm.
type slowStartState struct {
	ns, nr, cwnd, thresh, nacks, ntx uint16
}

// ctlMsg encapsulates state for control message transmission,
// wrapping the basic ControlMessage with transport-specific
// metadata.
type ctlMsg struct {
	// The message for transmission
	msg ControlMessage
	// The current retry count for the message.  This is bound by
	// the transport config MaxRetries parameter.
	nretries uint
	// When transmission is complete (either: the message has been
	// transmitted and acked by the peer, transmission itself has
	// failed, or retransmission has timed out) the resulting error
	// is sent on this channel.
	completeChan chan error
	// Completion state flag used internally by the transport.
	isComplete bool
	// Timer for retransmission if the peer doesn't ack the message.
	retryTimer *time.Timer
}

// rawMsg represents a raw frame read from the transport socket.
type rawMsg struct {
	b  []byte
	sa unix.Sockaddr
}

// TransportConfig represents the tunable parameters governing
// the behaviour of the reliable transport algorithm.
type TransportConfig struct {
	// Duration to wait after last message receipt before
	// sending a HELLO keepalive message.  If set to 0, no HELLO messages
	// are transmitted.
	HelloTimeout time.Duration
	// Maximum number of messages we will send to the peer without having
	// received an acknowledgement.
	TxWindowSize uint16
	// Maximum number of retransmits of an unacknowledged control packet.
	MaxRetries uint
	// Duration to wait before first packet retransmit.
	// Subsequent retransmits up to the limit set by maxRetries occur at
	// exponentially increasing intervals as per RFC3931.  If set to 0,
	// a default value of 1 second is used.
	RetryTimeout time.Duration
	// Duration to wait before explicitly acking a control message.
	// Most control messages will be implicitly acked by control protocol
	// responses.
	AckTimeout time.Duration
	// Version of the L2TP protocol to use for transport-generated messages.
	Version ProtocolVersion
	// Peer control connection ID to use for transport-generated messages
	PeerControlConnID ControlConnID
}

// Transport represents the RFC2661/RFC3931
// reliable transport algorithm state.
type Transport struct {
	slowStart            slowStartState
	config               TransportConfig
	cp                   *l2tpControlPlane
	helloTimer, ackTimer *time.Timer
	sendChan             chan *ctlMsg
	retryChan            chan *ctlMsg
	recvChan             chan ControlMessage
	cpChan               chan *rawMsg
	rxQueue              []ControlMessage
	txQueue, ackQueue    []*ctlMsg
	wg                   sync.WaitGroup
}

// Increment transport sequence number by one avoiding overflow
// as per RFC2661/RFC3931
func seqIncrement(seqNum uint16) uint16 {
	next := uint32(seqNum)
	next = (next + 1) % 0x10000
	return uint16(next)
}

// Sequence number comparision as per RFC2661/RFC3931
func seqCompare(seq1, seq2 uint16) int {
	var delta uint16
	if seq2 <= seq1 {
		delta = seq1 - seq2
	} else {
		delta = (0xffff - seq2 + 1) + seq1
	}
	if delta == 0 {
		return 0
	} else if delta < 0x8000 {
		return 1
	}
	return -1
}

func (s *slowStartState) reset(txWindow uint16) {
	s.cwnd = 1
	s.thresh = txWindow
	s.nacks = 0
	s.ntx = 0
}

func (s *slowStartState) canSend() bool {
	return s.ntx < s.cwnd
}

func (s *slowStartState) onSend() {
	if !s.canSend() {
		panic("slowStartState onSend() called when tx window is closed")
	}
	s.ntx++
}

func (s *slowStartState) onAck(maxTxWindow uint16) {
	if s.ntx > 0 {
		if s.cwnd < maxTxWindow {
			if s.cwnd < s.thresh {
				// slow start
				s.cwnd++
			} else {
				// congestion avoidance
				s.nacks++
				if s.nacks >= s.cwnd {
					s.nacks = 0
					s.cwnd++
				}
			}
		}
		s.ntx--
	}
	fmt.Printf("onAck(): window %d, ntx %d, thresh %d\n", s.cwnd, s.ntx, s.thresh)
}

func (s *slowStartState) onRetransmit() {
	s.thresh = s.cwnd / 2
	s.cwnd = 1
}

func (s *slowStartState) incrementNr() {
	s.nr = seqIncrement(s.nr)
}

func (s *slowStartState) incrementNs() {
	s.ns = seqIncrement(s.ns)
}

// A message with ns value equal to our nr is the next packet in sequence.
func (s *slowStartState) msgIsInSequence(msg ControlMessage) bool {
	return seqCompare(s.nr, msg.Ns()) == 0
}

// A message with ns value < our nr is stale/duplicated.
func (s *slowStartState) msgIsStale(msg ControlMessage) bool {
	return seqCompare(msg.Ns(), s.nr) == -1
}

func (m *ctlMsg) completeMessageSend(err error) {
	if !m.isComplete {
		m.isComplete = true
		if m.retryTimer != nil {
			m.retryTimer.Stop()
		}
		fmt.Printf("completeMessageSend(): %v complete\n", *m)
		m.completeChan <- err
	}
}

func newTimer(duration time.Duration) *time.Timer {
	if duration == 0 {
		duration = 1 * time.Hour
	}
	t := time.NewTimer(duration)
	t.Stop()
	return t
}

func sanitiseConfig(cfg *TransportConfig) {
	if cfg.TxWindowSize == 0 || cfg.TxWindowSize > 65535 {
		cfg.TxWindowSize = DefaultTransportConfig().TxWindowSize
	}
	if cfg.RetryTimeout == 0 {
		cfg.RetryTimeout = DefaultTransportConfig().RetryTimeout
	}
	if cfg.AckTimeout == 0 {
		cfg.AckTimeout = DefaultTransportConfig().AckTimeout
	}
}

func cpRead(xport *Transport, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		b := make([]byte, 4096)
		n, sa, err := xport.cp.readFrom(b)
		if err != nil {
			close(xport.cpChan)
			fmt.Printf("cpRead(%p): error reading from socket: %v\n", xport, err)
			return
		}
		fmt.Printf("cpRead(%p): read %d bytes\n", xport, n)
		xport.cpChan <- &rawMsg{b: b[:n], sa: sa}
	}
}

func runTransport(xport *Transport, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		fmt.Printf("runTransport(%p): enter select\n", xport)
		select {
		// Transmission request from user code
		case ctlMsg, ok := <-xport.sendChan:
			if !ok {
				xport.down(errors.New("transport shut down by user"))
				return
			}

			fmt.Printf("runTransport(%p) send msg %v\n", xport, ctlMsg)

			xport.txQueue = append(xport.txQueue, ctlMsg)
			err := xport.processTxQueue()
			if err != nil {
				xport.down(err)
				return
			}

		// Socket receive from the transport socket
		case rawMsg, ok := <-xport.cpChan:

			if !ok {
				xport.down(errors.New("control plane socket read error"))
				return
			}

			fmt.Printf("runTransport(%p) socket recv\n", xport)
			messages, err := xport.recvFrame(rawMsg)
			if err != nil {
				// Early packet handling can fail if we fail to parse a message or
				// the parsed message sequence number checks fail.  We ignore these
				// errors.
				// TODO: log the error here?
				break
			}

			for _, msg := range messages {
				xport.rxQueue = append(xport.rxQueue, msg)

				// Process the ack queue using sequence numbers from the newly received
				// message.  If we manage to dequeue a message it may result in opening
				// the window for further transmits.
				if xport.processAckQueue(msg) {
					err = xport.processTxQueue()
					if err != nil {
						xport.down(err)
						return
					}
				}
			}

			// TODO: connect socket here if required

			// Having added messages to the receive queue, process the queue
			// to attempt to handle any messages that are in sequence.
			xport.processRxQueue()

		// Message retry request due to timeout waiting for an ack
		case ctlMsg, ok := <-xport.retryChan:
			if !ok {
				return
			}

			fmt.Printf("runTransport(%p) retry timeout %v\n", xport, *ctlMsg)
			// It's possible that a message ack could race with the retry timer.
			// Hence we track completion state in the message struct to avoid
			// a bogus retransmit.
			if !ctlMsg.isComplete {
				err := xport.retransmitMessage(ctlMsg)
				if err != nil {
					ctlMsg.completeMessageSend(err)
					xport.down(err)
					return
				}
			}

		// Timer fired for sending a hello message
		case <-xport.helloTimer.C:
			fmt.Printf("runTransport(%p) HELLO timeout\n", xport)
			err := xport.sendHelloMessage()
			if err != nil {
				xport.down(err)
				return
			}

		// Timer fired for sending an explicit ack
		case <-xport.ackTimer.C:
			fmt.Printf("runTransport(%p) ACK timeout\n", xport)
			err := xport.sendExplicitAck()
			if err != nil {
				xport.down(err)
				return
			}
		}
	}
}

func (xport *Transport) recvFrame(rawMsg *rawMsg) (messages []ControlMessage, err error) {
	messages, err = ParseMessageBuffer(rawMsg.b)
	if err != nil {
		return nil, err
	}

	for _, msg := range messages {
		// Sanity check the packet sequence number: enqueue the packet for rx if it's OK
		if seqCompare(msg.Nr(), seqIncrement(xport.slowStart.ns)) > 0 {
			return nil, fmt.Errorf("dropping invalid packet %s ns %d nr %d (transport ns %d nr %d)",
				msg.Type(), msg.Ns(), msg.Nr(), xport.slowStart.ns, xport.slowStart.nr)
		}
	}

	return messages, nil
}

func (xport *Transport) recvMessage(msg ControlMessage) {
	if xport.slowStart.msgIsInSequence(msg) {
		xport.toggleAckTimer(true)
		xport.resetHelloTimer()
		xport.slowStart.incrementNr()
		xport.recvChan <- msg
	} else if xport.slowStart.msgIsStale(msg) {
		_ = xport.sendExplicitAck()
	}
}

func (xport *Transport) dequeueRxMessage() bool {
	for i, msg := range xport.rxQueue {
		if xport.slowStart.msgIsInSequence(msg) || xport.slowStart.msgIsStale(msg) {
			// Remove the message from the rx queue, and bubble it up for
			// processing.
			// In general we don't need to do anything more with an ack message
			// since they're just for the transport's purposes, so just drop them
			xport.rxQueue = append(xport.rxQueue[:i], xport.rxQueue[i+1:]...)
			if msg.Type() != AvpMsgTypeAck {
				xport.recvMessage(msg)
			}
			return true
		}
	}
	return false
}

func (xport *Transport) processRxQueue() {
	// Loop the receive queue looking for messages in sequence.
	// We give up once we've been through the queue without finding
	// an in-sequence message.
	for {
		if !xport.dequeueRxMessage() {
			return
		}
	}
}

func (xport *Transport) sendMessage1(msg ControlMessage, isRetransmit bool) error {
	// Set message sequence numbers.
	// A retransmitted message should have ns set already.
	if isRetransmit {
		msg.SetTransportSeqNum(msg.Ns(), xport.slowStart.nr)
	} else {
		msg.SetTransportSeqNum(xport.slowStart.ns, xport.slowStart.nr)
	}

	// Render as a byte slice and send.
	b, err := msg.ToBytes()
	if err == nil {
		_, err = xport.cp.write(b)
	}
	return err
}

// Exponential retry timeout scaling as per RFC2661/RFC3931
func (xport *Transport) scaleRetryTimeout(msg *ctlMsg) time.Duration {
	return xport.config.RetryTimeout * (1 << msg.nretries)
}

func (xport *Transport) sendMessage(msg *ctlMsg) error {

	err := xport.sendMessage1(msg.msg, msg.nretries > 0)
	if err == nil {
		xport.toggleAckTimer(false) // we have just sent an implicit ack
		xport.resetHelloTimer()
		if msg.msg.Type() != AvpMsgTypeAck && msg.nretries == 0 {
			xport.slowStart.incrementNs()
		}
		msg.retryTimer = time.AfterFunc(xport.scaleRetryTimeout(msg), func() {
			xport.retryChan <- msg
		})
	}
	return err
}

func (xport *Transport) retransmitMessage(msg *ctlMsg) error {
	msg.nretries++
	if msg.nretries >= xport.config.MaxRetries {
		return fmt.Errorf("transmit of %s failed after %d retry attempts",
			msg.msg.Type(), xport.config.MaxRetries)
	}
	err := xport.sendMessage(msg)
	if err == nil {
		xport.slowStart.onRetransmit()
	}
	return err
}

func (xport *Transport) processTxQueue() error {
	// Loop the transmit queue sending messages in order while
	// the transmit window is open.
	for i, msg := range xport.txQueue {
		if !xport.slowStart.canSend() {
			// We've sent all we can for the time being.  This is not
			// an error condition, so return successfully.
			return nil
		}

		// Remove from the tx queue, send, add to the ack queue
		xport.txQueue = append(xport.txQueue[:i], xport.txQueue[i+1:]...)
		err := xport.sendMessage(msg)
		if err == nil {
			xport.ackQueue = append(xport.ackQueue, msg)
			xport.slowStart.onSend()
		} else {
			msg.completeMessageSend(err)
			return err
		}
	}
	return nil
}

func (xport *Transport) processAckQueue(recvd ControlMessage) bool {
	found := false
	for i, msg := range xport.ackQueue {
		if seqCompare(recvd.Nr(), msg.msg.Ns()) > 0 {
			xport.slowStart.onAck(xport.config.TxWindowSize)
			xport.ackQueue = append(xport.ackQueue[:i], xport.ackQueue[i+1:]...)
			msg.completeMessageSend(nil)
			found = true
		}
	}
	return found
}

func (xport *Transport) down(err error) {

	fmt.Printf("xport(%p) is down: %v\n", xport, err)

	// Flush rx queue
	for i := range xport.rxQueue {
		xport.rxQueue = append(xport.rxQueue[:i], xport.rxQueue[i+1:]...)
	}

	// Flush tx and ack queues: complete these messages to unblock
	// callers pending on their completion.
	for i, msg := range xport.txQueue {
		xport.txQueue = append(xport.txQueue[:i], xport.txQueue[i+1:]...)
		msg.completeMessageSend(err)
	}

	for i, msg := range xport.ackQueue {
		xport.ackQueue = append(xport.ackQueue[:i], xport.ackQueue[i+1:]...)
		msg.completeMessageSend(err)
	}

	// Stop timers: we don't care about the return value since
	// the transport goroutine will return after calling this function
	// and hence won't be able to process racing timer messages
	xport.toggleAckTimer(false)
	_ = xport.helloTimer.Stop()

	// TODO: log error (probably)
	_ = err

	// Unblock recv path
	close(xport.recvChan)

	// Unblock control plane read goroutine
	xport.cp.close()
}

func (xport *Transport) toggleAckTimer(enable bool) {
	if enable {
		xport.ackTimer.Reset(xport.config.AckTimeout)
	} else {
		// TODO: is this bad?
		_ = xport.ackTimer.Stop()
	}
}

func (xport *Transport) resetHelloTimer() {
	if xport.config.HelloTimeout > 0 {
		xport.helloTimer.Reset(xport.config.HelloTimeout)
	}
}

func (xport *Transport) sendHelloMessage() error {
	return errors.New("Transport.sendHelloMessage not implemented")
}

func (xport *Transport) sendExplicitAck() (err error) {
	var msg ControlMessage

	if xport.config.Version == ProtocolVersion3Fallback || xport.config.Version == ProtocolVersion3 {
		avp, err := NewAvp(VendorIDIetf, AvpTypeMessage, AvpMsgTypeAck)
		if err != nil {
			return fmt.Errorf("failed to build v3 explicit ack message type AVP: %v", err)
		}
		msg, err = NewV3ControlMessage(xport.config.PeerControlConnID, []AVP{*avp})
		if err != nil {
			return fmt.Errorf("failed to build v3 explicit ack message: %v", err)
		}
	} else {
		msg, err = NewV2ControlMessage(xport.config.PeerControlConnID, 0, []AVP{})
		if err != nil {
			return fmt.Errorf("failed to build v2 ZLB message: %v", err)
		}
	}
	return xport.sendMessage1(msg, false)
}

// DefaultTransportConfig returns a default configuration for the transport.
func DefaultTransportConfig() TransportConfig {
	return TransportConfig{
		HelloTimeout: 0 * time.Second,
		TxWindowSize: 4,
		MaxRetries:   3,
		RetryTimeout: 1 * time.Second,
		AckTimeout:   100 * time.Millisecond,
		Version:      ProtocolVersion3,
	}
}

// NewTransport creates a new RFC2661/RFC3931 reliable transport.
// The control plane passed in is owned by the transport and will
// be closed by the transport when the transport is closed.
func NewTransport(cp *l2tpControlPlane, cfg TransportConfig) (xport *Transport, err error) {

	if cp == nil {
		return nil, errors.New("illegal nil control plane argument")
	}

	// Make sure the config is sane
	sanitiseConfig(&cfg)

	slowStart := slowStartState{}
	slowStart.reset(cfg.TxWindowSize)

	// We always create timer instances even if they're not going to be used.
	// This makes the logic for the transport go routine select easier to manage.
	helloTimer := newTimer(cfg.HelloTimeout)
	ackTimer := newTimer(cfg.AckTimeout)

	xport = &Transport{
		slowStart:  slowStart,
		config:     cfg,
		cp:         cp,
		helloTimer: helloTimer,
		ackTimer:   ackTimer,
		sendChan:   make(chan *ctlMsg),
		retryChan:  make(chan *ctlMsg),
		recvChan:   make(chan ControlMessage),
		cpChan:     make(chan *rawMsg),
		rxQueue:    []ControlMessage{},
		txQueue:    []*ctlMsg{},
		ackQueue:   []*ctlMsg{},
	}

	xport.wg.Add(2)
	go runTransport(xport, &xport.wg)
	go cpRead(xport, &xport.wg)

	return xport, nil
}

// Reconfigure allows transport parameters to be tweaked.
// Out of range values are automatically reset to sane default values.
func (xport *Transport) Reconfigure(cfg TransportConfig) {
	sanitiseConfig(&cfg)
	xport.config = cfg
}

// GetConfig allows transport parameters to be queried.
func (xport *Transport) GetConfig() TransportConfig {
	return xport.config
}

// Send sends a control message using the reliable transport.
// The caller will block until the message has been acked by the peer.
// Failure indicates that the transport has failed and the parent tunnel
// should be torn down.
func (xport *Transport) Send(msg ControlMessage) error {
	cm := ctlMsg{
		msg:          msg,
		nretries:     0,
		completeChan: make(chan error),
		isComplete:   false,
		retryTimer:   nil,
	}
	xport.sendChan <- &cm
	err := <-cm.completeChan
	return err
}

// Recv receives a control message using the reliable transport.
// The caller will block until a message has been received from the peer.
// Failure indicates that the transport has failed and the parent tunnel
// should be torn down.
func (xport *Transport) Recv() (msg ControlMessage, err error) {
	msg, ok := <-xport.recvChan
	if !ok {
		return nil, errors.New("transport is down")
	}
	return msg, nil
}

// Close closes the transport.
func (xport *Transport) Close() {
	close(xport.sendChan)
	xport.cp.close()
	xport.wg.Wait()
}
