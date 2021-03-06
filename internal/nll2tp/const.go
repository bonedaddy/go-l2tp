// TODO

// WARNING: This file has automatically been generated on Wed, 12 Feb 2020 10:38:25 UTC.
// Code generated by https://git.io/c-for-go. DO NOT EDIT.

package nll2tp

const (
	// CmdMax as defined in nll2tp/l2tp.h:90
	CmdMax = -1
	// AttrMax as defined in nll2tp/l2tp.h:135
	AttrMax = -1
	// AttrStatsMax as defined in nll2tp/l2tp.h:152
	AttrStatsMax = -1
	// GenlName as defined in nll2tp/l2tp.h:198
	GenlName = "l2tp"
	// GenlVersion as defined in nll2tp/l2tp.h:199
	GenlVersion = 0x1
	// GenlMcgroup as defined in nll2tp/l2tp.h:200
	GenlMcgroup = "l2tp"
)

const (
	// CmdNoop as declared in nll2tp/l2tp.h:78
	CmdNoop = iota
	// CmdTunnelCreate as declared in nll2tp/l2tp.h:79
	CmdTunnelCreate = 1
	// CmdTunnelDelete as declared in nll2tp/l2tp.h:80
	CmdTunnelDelete = 2
	// CmdTunnelModify as declared in nll2tp/l2tp.h:81
	CmdTunnelModify = 3
	// CmdTunnelGet as declared in nll2tp/l2tp.h:82
	CmdTunnelGet = 4
	// CmdSessionCreate as declared in nll2tp/l2tp.h:83
	CmdSessionCreate = 5
	// CmdSessionDelete as declared in nll2tp/l2tp.h:84
	CmdSessionDelete = 6
	// CmdSessionModify as declared in nll2tp/l2tp.h:85
	CmdSessionModify = 7
	// CmdSessionGet as declared in nll2tp/l2tp.h:86
	CmdSessionGet = 8
)

const (
	// AttrNone as declared in nll2tp/l2tp.h:96
	AttrNone = iota
	// AttrPwType as declared in nll2tp/l2tp.h:97
	AttrPwType = 1
	// AttrEncapType as declared in nll2tp/l2tp.h:98
	AttrEncapType = 2
	// AttrOffset as declared in nll2tp/l2tp.h:99
	AttrOffset = 3
	// AttrDataSeq as declared in nll2tp/l2tp.h:100
	AttrDataSeq = 4
	// AttrL2specType as declared in nll2tp/l2tp.h:101
	AttrL2specType = 5
	// AttrL2specLen as declared in nll2tp/l2tp.h:102
	AttrL2specLen = 6
	// AttrProtoVersion as declared in nll2tp/l2tp.h:103
	AttrProtoVersion = 7
	// AttrIfname as declared in nll2tp/l2tp.h:104
	AttrIfname = 8
	// AttrConnId as declared in nll2tp/l2tp.h:105
	AttrConnId = 9
	// AttrPeerConnId as declared in nll2tp/l2tp.h:106
	AttrPeerConnId = 10
	// AttrSessionId as declared in nll2tp/l2tp.h:107
	AttrSessionId = 11
	// AttrPeerSessionId as declared in nll2tp/l2tp.h:108
	AttrPeerSessionId = 12
	// AttrUdpCsum as declared in nll2tp/l2tp.h:109
	AttrUdpCsum = 13
	// AttrVlanId as declared in nll2tp/l2tp.h:110
	AttrVlanId = 14
	// AttrCookie as declared in nll2tp/l2tp.h:111
	AttrCookie = 15
	// AttrPeerCookie as declared in nll2tp/l2tp.h:112
	AttrPeerCookie = 16
	// AttrDebug as declared in nll2tp/l2tp.h:113
	AttrDebug = 17
	// AttrRecvSeq as declared in nll2tp/l2tp.h:114
	AttrRecvSeq = 18
	// AttrSendSeq as declared in nll2tp/l2tp.h:115
	AttrSendSeq = 19
	// AttrLnsMode as declared in nll2tp/l2tp.h:116
	AttrLnsMode = 20
	// AttrUsingIpsec as declared in nll2tp/l2tp.h:117
	AttrUsingIpsec = 21
	// AttrRecvTimeout as declared in nll2tp/l2tp.h:118
	AttrRecvTimeout = 22
	// AttrFd as declared in nll2tp/l2tp.h:119
	AttrFd = 23
	// AttrIpSaddr as declared in nll2tp/l2tp.h:120
	AttrIpSaddr = 24
	// AttrIpDaddr as declared in nll2tp/l2tp.h:121
	AttrIpDaddr = 25
	// AttrUdpSport as declared in nll2tp/l2tp.h:122
	AttrUdpSport = 26
	// AttrUdpDport as declared in nll2tp/l2tp.h:123
	AttrUdpDport = 27
	// AttrMtu as declared in nll2tp/l2tp.h:124
	AttrMtu = 28
	// AttrMru as declared in nll2tp/l2tp.h:125
	AttrMru = 29
	// AttrStats as declared in nll2tp/l2tp.h:126
	AttrStats = 30
	// AttrIp6Saddr as declared in nll2tp/l2tp.h:127
	AttrIp6Saddr = 31
	// AttrIp6Daddr as declared in nll2tp/l2tp.h:128
	AttrIp6Daddr = 32
	// AttrUdpZeroCsum6Tx as declared in nll2tp/l2tp.h:129
	AttrUdpZeroCsum6Tx = 33
	// AttrUdpZeroCsum6Rx as declared in nll2tp/l2tp.h:130
	AttrUdpZeroCsum6Rx = 34
	// AttrPad as declared in nll2tp/l2tp.h:131
	AttrPad = 35
)

const (
	// AttrStatsNone as declared in nll2tp/l2tp.h:139
	AttrStatsNone = iota
	// AttrTxPackets as declared in nll2tp/l2tp.h:140
	AttrTxPackets = 1
	// AttrTxBytes as declared in nll2tp/l2tp.h:141
	AttrTxBytes = 2
	// AttrTxErrors as declared in nll2tp/l2tp.h:142
	AttrTxErrors = 3
	// AttrRxPackets as declared in nll2tp/l2tp.h:143
	AttrRxPackets = 4
	// AttrRxBytes as declared in nll2tp/l2tp.h:144
	AttrRxBytes = 5
	// AttrRxSeqDiscards as declared in nll2tp/l2tp.h:145
	AttrRxSeqDiscards = 6
	// AttrRxOosPackets as declared in nll2tp/l2tp.h:146
	AttrRxOosPackets = 7
	// AttrRxErrors as declared in nll2tp/l2tp.h:147
	AttrRxErrors = 8
	// AttrStatsPad as declared in nll2tp/l2tp.h:148
	AttrStatsPad = 9
)

// L2tpPwtype as declared in nll2tp/l2tp.h:154
type L2tpPwtype int32

// L2tpPwtype enumeration from nll2tp/l2tp.h:154
const (
	PwtypeNone    = 0x0000
	PwtypeEthVlan = 0x0004
	PwtypeEth     = 0x0005
	PwtypePpp     = 0x0007
	PwtypePppAc   = 0x0008
	PwtypeIp      = 0x000b
)

// L2tpL2specType as declared in nll2tp/l2tp.h:164
type L2tpL2specType int32

// L2tpL2specType enumeration from nll2tp/l2tp.h:164
const (
	L2spectypeNone    = iota
	L2spectypeDefault = 1
)

// L2tpEncapType as declared in nll2tp/l2tp.h:169
type L2tpEncapType int32

// L2tpEncapType enumeration from nll2tp/l2tp.h:169
const (
	EncaptypeUdp = iota
	EncaptypeIp  = 1
)

// L2tpSeqmode as declared in nll2tp/l2tp.h:174
type L2tpSeqmode int32

// L2tpSeqmode enumeration from nll2tp/l2tp.h:174
const (
	SeqNone = iota
	SeqIp   = 1
	SeqAll  = 2
)

// L2tpDebugFlags as declared in nll2tp/l2tp.h:188
type L2tpDebugFlags uint32

// L2tpDebugFlags enumeration from nll2tp/l2tp.h:188
const (
	MsgDebug   = (1 << 0)
	MsgControl = (1 << 1)
	MsgSeq     = (1 << 2)
	MsgData    = (1 << 3)
)
