package l2tp

import (
	"fmt"
	"net"

	"github.com/go-kit/kit/log"
	"github.com/katalix/go-l2tp/internal/nll2tp"
	"golang.org/x/sys/unix"
)

// Context is a container for a collection of L2TP tunnels and
// their sessions, and associated configuration.
type Context struct {
	logger  log.Logger
	nlconn  *nll2tp.Conn
	tunnels map[string]Tunnel
}

// ContextConfig encodes top-level configuration for an L2TP
// context.
type ContextConfig struct {
	// TODO
}

// Tunnel is an interface representing an L2TP tunnel.
type Tunnel interface {
	// NewSession adds a session to a tunnel instance.
	//
	// The name provided must be unique in the parent tunnel.
	NewSession(name string, cfg *SessionConfig) (Session, error)

	// Close closes the tunnel, releasing allocated resources.
	//
	// Any sessions instantiated inside the tunnel are removed.
	Close()

	getCfg() *TunnelConfig
	getNLConn() *nll2tp.Conn
	getLogger() log.Logger
	unlinkSession(name string)
}

// Session is an interface representing an L2TP session.
type Session interface {
	// Close closes the session, releasing allocated resources.
	Close()
}

// NewContext creates a new L2TP context, which can then be used
// to instantiate tunnel and session instances.
//
// Context creation will fail if it is not possible to connect to
// the Linux kernel L2TP subsystem using netlink, so the Linux
// kernel L2TP modules must be running.
//
// Logging is generated using go-kit levels: informational logging
// uses the Info level, while verbose debugging logging uses the
// Debug level.  Error conditions may be logged using the Error level
// depending on the tunnel type.
//
// If a nil logger is passed, all logging is disabled.
// If a nil configuration is passed, default configuration will
// be used.
func NewContext(logger log.Logger, cfg *ContextConfig) (*Context, error) {

	if logger == nil {
		logger = log.NewNopLogger()
	}

	if cfg == nil {
		// TODO: default configuration.
		// Eventually we might set things like host name, router ID,
		// etc, etc.
	}

	nlconn, err := nll2tp.Dial()
	if err != nil {
		return nil, fmt.Errorf("failed to establish a netlink/L2TP connection: %v", err)
	}

	return &Context{
		logger:  logger,
		nlconn:  nlconn,
		tunnels: make(map[string]Tunnel),
	}, nil
}

// NewQuiescentTunnel creates a new "quiescent" L2TP tunnel.
//
// A quiescent tunnel creates a user space socket for the
// L2TP control plane, but does not run the control protocol
// beyond acknowledging messages and optionally sending HELLO
// messages.
//
// The data plane is established on creation of the tunnel instance.
//
// The name provided must be unique in the Context.
//
// The tunnel configuration must include local and peer addresses
// and local and peer tunnel IDs.
func (ctx *Context) NewQuiescentTunnel(name string, cfg *TunnelConfig) (tunl Tunnel, err error) {

	var sal, sap unix.Sockaddr

	// Must have configuration
	if cfg == nil {
		return nil, fmt.Errorf("invalid nil config")
	}

	// Must not have name clashes
	if _, ok := ctx.tunnels[name]; ok {
		return nil, fmt.Errorf("already have tunnel %q", name)
	}

	// Sanity check the configuration
	if cfg.Version != ProtocolVersion3 && cfg.Encap == EncapTypeIP {
		return nil, fmt.Errorf("IP encapsulation only supported for L2TPv3 tunnels")
	}
	if cfg.Version == ProtocolVersion2 {
		if cfg.TunnelID == 0 || cfg.TunnelID > 65535 {
			return nil, fmt.Errorf("L2TPv2 connection ID %v out of range", cfg.TunnelID)
		} else if cfg.PeerTunnelID == 0 || cfg.PeerTunnelID > 65535 {
			return nil, fmt.Errorf("L2TPv2 peer connection ID %v out of range", cfg.PeerTunnelID)
		}
	} else {
		if cfg.TunnelID == 0 || cfg.PeerTunnelID == 0 {
			return nil, fmt.Errorf("L2TPv3 tunnel IDs %v and %v must both be > 0",
				cfg.TunnelID, cfg.PeerTunnelID)
		}
	}

	// Initialise tunnel address structures
	switch cfg.Encap {
	case EncapTypeUDP:
		sal, sap, err = newUDPAddressPair(cfg.Local, cfg.Peer)
	case EncapTypeIP:
		sal, sap, err = newIPAddressPair(cfg.Local, cfg.TunnelID,
			cfg.Peer, cfg.PeerTunnelID)
	default:
		err = fmt.Errorf("unrecognised encapsulation type %v", cfg.Encap)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to initialise tunnel addresses: %v", err)
	}

	tunl, err = newQuiescentTunnel(name, ctx, sal, sap, cfg)
	if err != nil {
		return nil, err
	}

	ctx.tunnels[name] = tunl

	return tunl, nil
}

// NewStaticTunnel creates a new static (unmanaged) L2TP tunnel.
//
// A static tunnel does not run any control protocol
// and instead merely instantiates the data plane in the
// kernel.  This is equivalent to the Linux 'ip l2tp'
// command(s).
//
// Static L2TPv2 tunnels are not practically useful,
// so NewStaticTunnel only supports creation of L2TPv3
// unmanaged tunnel instances.
//
// The name provided must be unique in the Context.
//
// The tunnel configuration must include local and peer addresses
// and local and peer tunnel IDs.
func (ctx *Context) NewStaticTunnel(name string, cfg *TunnelConfig) (tunl Tunnel, err error) {

	var sal, sap unix.Sockaddr

	// Must have configuration
	if cfg == nil {
		return nil, fmt.Errorf("invalid nil config")
	}

	// Must not have name clashes
	if _, ok := ctx.tunnels[name]; ok {
		return nil, fmt.Errorf("already have tunnel %q", name)
	}

	// Sanity check  the configuration
	if cfg.Version != ProtocolVersion3 {
		return nil, fmt.Errorf("static tunnels can be L2TPv3 only")
	}
	if cfg.TunnelID == 0 || cfg.PeerTunnelID == 0 {
		return nil, fmt.Errorf("L2TPv3 tunnel IDs %v and %v must both be > 0",
			cfg.TunnelID, cfg.PeerTunnelID)
	}

	// Initialise tunnel address structures
	switch cfg.Encap {
	case EncapTypeUDP:
		sal, sap, err = newUDPAddressPair(cfg.Local, cfg.Peer)
	case EncapTypeIP:
		sal, sap, err = newIPAddressPair(cfg.Local, cfg.TunnelID,
			cfg.Peer, cfg.PeerTunnelID)
	default:
		err = fmt.Errorf("unrecognised encapsulation type %v", cfg.Encap)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to initialise tunnel addresses: %v", err)
	}

	tunl, err = newStaticTunnel(name, ctx, sal, sap, cfg)
	if err != nil {
		return nil, err
	}

	ctx.tunnels[name] = tunl

	return tunl, nil
}

// Close tears down the context, including all the L2TP tunnels and sessions
// running inside it.
func (ctx *Context) Close() {
	for name, tunl := range ctx.tunnels {
		tunl.Close()
		ctx.unlinkTunnel(name)
	}
	ctx.nlconn.Close()
}

func (ctx *Context) unlinkTunnel(name string) {
	delete(ctx.tunnels, name)
}

func newUDPTunnelAddress(address string) (unix.Sockaddr, error) {

	u, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, fmt.Errorf("resolve %v: %v", address, err)
	}

	if b := u.IP.To4(); b != nil {
		return &unix.SockaddrInet4{
			Port: u.Port,
			Addr: [4]byte{b[0], b[1], b[2], b[3]},
		}, nil
	} else if b := u.IP.To16(); b != nil {
		// TODO: SockaddrInet6 has a uint32 ZoneId, while UDPAddr
		// has a Zone string.  How to convert between the two?
		return &unix.SockaddrInet6{
			Port: u.Port,
			Addr: [16]byte{
				b[0], b[1], b[2], b[3],
				b[4], b[5], b[6], b[7],
				b[8], b[9], b[10], b[11],
				b[12], b[13], b[14], b[15],
			},
			// ZoneId
		}, nil
	}

	return nil, fmt.Errorf("unhandled address family")
}

func newUDPAddressPair(local, remote string) (sal, sap unix.Sockaddr, err error) {
	sal, err = newUDPTunnelAddress(local)
	if err != nil {
		return nil, nil, err
	}
	sap, err = newUDPTunnelAddress(remote)
	if err != nil {
		return nil, nil, err
	}
	return sal, sap, nil
}

func newIPTunnelAddress(address string, ccid ControlConnID) (unix.Sockaddr, error) {

	u, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, fmt.Errorf("resolve %v: %v", address, err)
	}

	if b := u.IP.To4(); b != nil {
		return &unix.SockaddrL2TPIP{
			Addr:   [4]byte{b[0], b[1], b[2], b[3]},
			ConnId: uint32(ccid),
		}, nil
	} else if b := u.IP.To16(); b != nil {
		// TODO: SockaddrInet6 has a uint32 ZoneId, while UDPAddr
		// has a Zone string.  How to convert between the two?
		return &unix.SockaddrL2TPIP6{
			Addr: [16]byte{
				b[0], b[1], b[2], b[3],
				b[4], b[5], b[6], b[7],
				b[8], b[9], b[10], b[11],
				b[12], b[13], b[14], b[15],
			},
			// ZoneId
			ConnId: uint32(ccid),
		}, nil
	}

	return nil, fmt.Errorf("unhandled address family")
}

func newIPAddressPair(local string, ccid ControlConnID, remote string, pccid ControlConnID) (sal, sap unix.Sockaddr, err error) {
	sal, err = newIPTunnelAddress(local, ccid)
	if err != nil {
		return nil, nil, err
	}
	sap, err = newIPTunnelAddress(remote, pccid)
	if err != nil {
		return nil, nil, err
	}
	return sal, sap, nil
}
