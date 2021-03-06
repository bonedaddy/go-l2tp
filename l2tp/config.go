package l2tp

import (
	"fmt"
	"time"

	"github.com/pelletier/go-toml"
)

// Config represents L2TP configuration for tunnel and session instances,
// and uses the TOML format: https://github.com/toml-lang/toml.
type Config struct {
	// entire tree as a map
	cm map[string]interface{}
	// map of tunnels mapping tunnel name to config
	tunnels map[string]*TunnelConfig
}

// TunnelConfig encapsulates tunnel configuration for a single
// connection between two L2TP hosts.  Each tunnel may contain
// multiple sessions.
type TunnelConfig struct {
	Local        string
	Peer         string
	Encap        EncapType
	Version      ProtocolVersion
	TunnelID     ControlConnID
	PeerTunnelID ControlConnID
	WindowSize   uint16
	HelloTimeout time.Duration
	RetryTimeout time.Duration
	MaxRetries   uint
	// map of sessions within the tunnel
	Sessions map[string]*SessionConfig
}

// SessionConfig encapsulates session configuration for a pseudowire
// connection within a tunnel between two L2TP hosts.
type SessionConfig struct {
	SessionID      ControlConnID
	PeerSessionID  ControlConnID
	Pseudowire     PseudowireType
	SeqNum         bool
	ReorderTimeout time.Duration
	Cookie         []byte
	PeerCookie     []byte
	InterfaceName  string
	L2SpecType     L2SpecType
}

func toBool(v interface{}) (bool, error) {
	if b, ok := v.(bool); ok {
		return b, nil
	}
	return false, fmt.Errorf("supplied value could not be parsed as a bool")
}

// go-toml's ToMap function represents numbers as either uint64 or int64.
// So when we are converting numbers, we need to figure out which one it
// has picked and range check to ensure that the number from the config
// fits within the range of the destination type.
func toByte(v interface{}) (byte, error) {
	if b, ok := v.(int64); ok {
		if b < 0x0 || b > 0xff {
			return 0, fmt.Errorf("value %x out of range", b)
		}
		return byte(b), nil
	} else if b, ok := v.(uint64); ok {
		if b > 0xff {
			return 0, fmt.Errorf("value %x out of range", b)
		}
		return byte(b), nil
	}
	return 0, fmt.Errorf("unexpected %T value %v", v, v)
}

func toUint16(v interface{}) (uint16, error) {
	if b, ok := v.(int64); ok {
		if b < 0x0 || b > 0xffff {
			return 0, fmt.Errorf("value %x out of range", b)
		}
		return uint16(b), nil
	} else if b, ok := v.(uint64); ok {
		if b > 0xffff {
			return 0, fmt.Errorf("value %x out of range", b)
		}
		return uint16(b), nil
	}
	return 0, fmt.Errorf("unexpected %T value %v", v, v)
}

func toUint32(v interface{}) (uint32, error) {
	if b, ok := v.(int64); ok {
		if b < 0x0 || b > 0xffffffff {
			return 0, fmt.Errorf("value %x out of range", b)
		}
		return uint32(b), nil
	} else if b, ok := v.(uint64); ok {
		if b > 0xffffffff {
			return 0, fmt.Errorf("value %x out of range", b)
		}
		return uint32(b), nil
	}
	return 0, fmt.Errorf("unexpected %T value %v", v, v)
}

func toString(v interface{}) (string, error) {
	if s, ok := v.(string); ok {
		return s, nil
	}
	return "", fmt.Errorf("supplied value could not be parsed as a string")
}

func toDurationMs(v interface{}) (time.Duration, error) {
	u, err := toUint32(v)
	return time.Duration(u) * time.Millisecond, err
}

func toVersion(v interface{}) (ProtocolVersion, error) {
	s, err := toString(v)
	if err == nil {
		switch s {
		case "l2tpv2":
			return ProtocolVersion2, nil
		case "l2tpv3":
			return ProtocolVersion3, nil
		}
		return 0, fmt.Errorf("expect 'l2tpv2' or 'l2tpv3'")
	}
	return 0, err
}

func toEncapType(v interface{}) (EncapType, error) {
	s, err := toString(v)
	if err == nil {
		switch s {
		case "udp":
			return EncapTypeUDP, nil
		case "ip":
			return EncapTypeIP, nil
		}
		return 0, fmt.Errorf("expect 'udp' or 'ip'")
	}
	return 0, err
}

func toPseudowireType(v interface{}) (PseudowireType, error) {
	s, err := toString(v)
	if err == nil {
		switch s {
		case "ppp":
			return PseudowireTypePPP, nil
		case "eth":
			return PseudowireTypeEth, nil
		}
		return 0, fmt.Errorf("expect 'ppp' or 'eth'")
	}
	return 0, err
}

func toL2SpecType(v interface{}) (L2SpecType, error) {
	s, err := toString(v)
	if err == nil {
		switch s {
		case "none":
			return L2SpecTypeNone, nil
		case "default":
			return L2SpecTypeDefault, nil
		}
		return 0, fmt.Errorf("expect 'none' or 'default'")
	}
	return L2SpecTypeNone, err
}

func toCCID(v interface{}) (ControlConnID, error) {
	u, err := toUint32(v)
	return ControlConnID(u), err
}

func toBytes(v interface{}) ([]byte, error) {
	out := []byte{}

	// First ensure that the supplied value is actually an array
	numbers, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected array value")
	}

	// TOML arrays can be mixed type, so we have to check on a value-by-value
	// basis that the value in the array can be represented as a byte.
	for _, number := range numbers {
		b, err := toByte(number)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

func newSessionConfig(scfg map[string]interface{}) (*SessionConfig, error) {
	sc := SessionConfig{}
	for k, v := range scfg {
		var err error
		switch k {
		case "sid":
			sc.SessionID, err = toCCID(v)
		case "psid":
			sc.PeerSessionID, err = toCCID(v)
		case "pseudowire":
			sc.Pseudowire, err = toPseudowireType(v)
		case "seqnum":
			sc.SeqNum, err = toBool(v)
		case "reorder_timeout":
			sc.ReorderTimeout, err = toDurationMs(v)
		case "cookie":
			sc.Cookie, err = toBytes(v)
		case "peer_cookie":
			sc.PeerCookie, err = toBytes(v)
		case "interface_name":
			sc.InterfaceName, err = toString(v)
		case "l2spec_type":
			sc.L2SpecType, err = toL2SpecType(v)
		default:
			return nil, fmt.Errorf("unrecognised parameter '%v'", k)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to process %v: %v", k, err)
		}
	}
	return &sc, nil
}

func (t *TunnelConfig) loadSessions(v interface{}) error {
	sessions, ok := v.(map[string]interface{})
	if !ok {
		return fmt.Errorf("session instances must be named, e.g. '[tunnel.mytunnel.session.mysession]'")
	}
	for name, got := range sessions {
		smap, ok := got.(map[string]interface{})
		if !ok {
			return fmt.Errorf("session instances must be named, e.g. '[tunnel.mytunnel.session.mysession]'")
		}
		scfg, err := newSessionConfig(smap)
		if err != nil {
			return fmt.Errorf("session %v: %v", name, err)
		}
		t.Sessions[name] = scfg
	}
	return nil
}

func newTunnelConfig(tcfg map[string]interface{}) (*TunnelConfig, error) {
	tc := TunnelConfig{
		Sessions: make(map[string]*SessionConfig),
	}
	for k, v := range tcfg {
		var err error
		switch k {
		case "local":
			tc.Local, err = toString(v)
		case "peer":
			tc.Peer, err = toString(v)
		case "encap":
			tc.Encap, err = toEncapType(v)
		case "version":
			tc.Version, err = toVersion(v)
		case "tid":
			tc.TunnelID, err = toCCID(v)
		case "ptid":
			tc.PeerTunnelID, err = toCCID(v)
		case "window_size":
			tc.WindowSize, err = toUint16(v)
		case "hello_timeout":
			tc.HelloTimeout, err = toDurationMs(v)
		case "retry_timeout":
			tc.RetryTimeout, err = toDurationMs(v)
		case "max_retries":
			if u, err := toUint16(v); err == nil {
				tc.MaxRetries = uint(u)
			}
		case "session":
			err = tc.loadSessions(v)
		default:
			return nil, fmt.Errorf("unrecognised parameter '%v'", k)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to process %v: %v", k, err)
		}
	}
	return &tc, nil
}

func (cfg *Config) loadTunnels() error {
	var tunnels map[string]interface{}

	// Extract the tunnel map from the configuration tree
	if got, ok := cfg.cm["tunnel"]; ok {
		tunnels, ok = got.(map[string]interface{})
		if !ok {
			return fmt.Errorf("tunnel instances must be named, e.g. '[tunnel.mytunnel]'")
		}
	} else {
		return fmt.Errorf("no tunnel table present")
	}

	// Iterate through the map and build tunnel config instances
	for name, got := range tunnels {
		tmap, ok := got.(map[string]interface{})
		if !ok {
			return fmt.Errorf("tunnel instances must be named, e.g. '[tunnel.mytunnel]'")
		}
		tcfg, err := newTunnelConfig(tmap)
		if err != nil {
			return fmt.Errorf("tunnel %v: %v", name, err)
		}
		cfg.tunnels[name] = tcfg
	}
	return nil
}

func newConfig(tree *toml.Tree) (*Config, error) {
	cfg := &Config{
		cm:      tree.ToMap(),
		tunnels: make(map[string]*TunnelConfig),
	}
	err := cfg.loadTunnels()
	if err != nil {
		return nil, fmt.Errorf("failed to parse tunnels: %v", err)
	}
	return cfg, nil
}

// LoadConfigFile loads configuration from the specified file.
func LoadConfigFile(path string) (*Config, error) {
	tree, err := toml.LoadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load config file: %v", err)
	}
	return newConfig(tree)
}

// LoadConfigString loads configuration from the specified string.
func LoadConfigString(content string) (*Config, error) {
	tree, err := toml.Load(content)
	if err != nil {
		return nil, fmt.Errorf("failed to load config string: %v", err)
	}
	return newConfig(tree)
}

// GetTunnels returns a map of tunnel name to tunnel config for
// all the tunnels described by the configuration.
func (cfg *Config) GetTunnels() map[string]*TunnelConfig {
	return cfg.tunnels
}

// ToMap provides access to the configuration for application-specific
// information to be handled.
func (cfg *Config) ToMap() map[string]interface{} {
	return cfg.cm
}
