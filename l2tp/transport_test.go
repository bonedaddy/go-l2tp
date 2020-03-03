package l2tp

import (
	"testing"
	"time"
)

func TestOpenClose(t *testing.T) {
	xport, err := NewTransport(nil, DefaultTransportConfig())
	if xport != nil {
		t.Fatalf("NewTransport() with nil controlplane succeeded")
	} else if err == nil {
		t.Fatalf("NewTransport() with nil controlplane didn't report error")
	}

	cp, err := newL2tpControlPlane("127.0.0.1:5000", "127.0.0.1:6000", false)
	if err != nil {
		t.Fatalf("newL2tpControlPlane() failed: %v", err)
	}

	xport, err = NewTransport(cp, DefaultTransportConfig())
	if xport == nil {
		t.Fatalf("NewTransport() returned nil controlplane")
	} else if err != nil {
		t.Fatalf("NewTransport() error")
	}

	// Sleep briefly to allow the go routines to get scheduled:
	// we want to at least run the code there to give us a chance
	// to trip over e.g. uninitialised fields
	time.Sleep(1 * time.Millisecond)

	xport.Close()
}

func TestSeqNumIncrement(t *testing.T) {
	cases := []struct {
		in, want uint16
	}{
		{uint16(0), uint16(1)},
		{uint16(65534), uint16(65535)},
		{uint16(65535), uint16(0)},
	}
	for _, c := range cases {
		got := seqIncrement(c.in)
		if got != c.want {
			t.Errorf("seqIncrement(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestSeqNumCompare(t *testing.T) {
	cases := []struct {
		seq1, seq2 uint16
		want       int
	}{
		{uint16(15), uint16(15), 0},
		{uint16(15), uint16(0), 1},
		{uint16(15), uint16(65535), 1},
		{uint16(15), uint16(32784), 1},
		{uint16(15), uint16(16), -1},
		{uint16(15), uint16(15000), -1},
		{uint16(15), uint16(32783), -1},
	}
	for _, c := range cases {
		got := seqCompare(c.seq1, c.seq2)
		if got != c.want {
			t.Errorf("seqCompare(%d, %d) = %d, want %d", c.seq1, c.seq2, got, c.want)
		}
	}
}

func checkWindowOpen(ss *slowStartState, t *testing.T) {
	if !ss.canSend() {
		t.Fatalf("transport window is closed when we expect it to be open")
	}
}

func checkWindowClosed(ss *slowStartState, t *testing.T) {
	if ss.canSend() {
		t.Fatalf("transport window is open when we expect it to be closed")
	}
}

func checkCwndThresh(ss *slowStartState, cwnd, thresh uint16, t *testing.T) {
	if ss.cwnd != cwnd {
		t.Fatalf("transport window didn't correctly reset on retransmission: expected %d, got %d", cwnd, ss.cwnd)
	}
	if ss.thresh != thresh {
		t.Fatalf("transport threshold didn't correctly reset on retransmission: expected %d, got %d", thresh, ss.thresh)
	}
}

func TestSlowStart(t *testing.T) {
	txWindow := uint16(4)

	// initialise state and validate window is open
	ss := slowStartState{}
	ss.reset(txWindow)
	checkWindowOpen(&ss, t)

	// send a packet, validate window is now closed
	ss.onSend()
	checkWindowClosed(&ss, t)

	// ack the packet: should now be able to send two packets before window closes
	ss.onAck(txWindow)
	for i := 0; i < 2; i++ {
		checkWindowOpen(&ss, t)
		ss.onSend()
	}
	checkWindowClosed(&ss, t)

	// ack the two packets in flight: should now be able to send four packets
	for i := 0; i < 2; i++ {
		ss.onAck(txWindow)
	}
	for i := 0; i < 4; i++ {
		checkWindowOpen(&ss, t)
		ss.onSend()
	}
	checkWindowClosed(&ss, t)

	// ack the four packets in flight, validate the state hasn't exceeded the max window
	for i := 0; i < 4; i++ {
		ss.onAck(txWindow)
		checkWindowOpen(&ss, t)
		if ss.cwnd > txWindow {
			t.Fatalf("transport window %d exceeded max %d", ss.cwnd, txWindow)
		}
	}

	// retransmit: validate threshold is reduced and cwnd is reset
	checkWindowOpen(&ss, t)
	ss.onSend()
	ss.onRetransmit()
	checkWindowClosed(&ss, t)
	checkCwndThresh(&ss, 1, 2, t)

	// ack the retransmit, validate we're in slow-start still
	ss.onAck(txWindow)
	checkWindowOpen(&ss, t)
	checkCwndThresh(&ss, 2, 2, t)

	// send packets, recv acks, validate congestion avoidance is applied
	checkWindowOpen(&ss, t)
	ss.onSend()
	ss.onAck(txWindow)
	checkCwndThresh(&ss, 2, 2, t)
	for i := 0; i < 3; i++ {
		checkWindowOpen(&ss, t)
		ss.onSend()
		ss.onAck(txWindow)
		checkCwndThresh(&ss, 3, 2, t)
	}
	checkWindowOpen(&ss, t)
	ss.onSend()
	ss.onAck(txWindow)
	checkCwndThresh(&ss, 4, 2, t)

	// lots more transmission, validate we don't exceed max tx window in congestion avoidance
	for i := 0; i < 100; i++ {
		checkWindowOpen(&ss, t)
		ss.onSend()
		ss.onAck(txWindow)
		checkCwndThresh(&ss, 4, 2, t)
	}

}