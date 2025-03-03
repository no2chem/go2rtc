package webrtc

import (
	"fmt"
	"github.com/AlexxIT/go2rtc/pkg/streamer"
	"github.com/pion/ice/v2"
	"github.com/pion/stun"
	"github.com/pion/webrtc/v3"
	"net"
	"strconv"
	"strings"
)

func NewCandidate(address string) (string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", err
	}

	i, err := strconv.Atoi(port)
	if err != nil {
		return "", err
	}

	cand, err := ice.NewCandidateHost(&ice.CandidateHostConfig{
		Network:   "tcp",
		Address:   host,
		Port:      i,
		Component: ice.ComponentRTP,
		TCPType:   ice.TCPTypePassive,
	})
	if err != nil {
		return "", err
	}

	return "candidate:" + cand.Marshal(), nil
}

func LookupIP(address string) (string, error) {
	if strings.HasPrefix(address, "stun:") {
		ip, err := GetPublicIP()
		if err != nil {
			return "", err
		}
		return ip.String() + address[4:], nil
	}

	if IsIP(address) {
		return address, nil
	}

	i := strings.IndexByte(address, ':')
	ips, err := net.LookupIP(address[:i])
	if err != nil {
		return "", err
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("can't resolve: %s", address)
	}

	return ips[0].String() + address[i:], nil
}

// GetPublicIP example from https://github.com/pion/stun
func GetPublicIP() (net.IP, error) {
	c, err := stun.Dial("udp", "stun.l.google.com:19302")
	if err != nil {
		return nil, err
	}

	var res stun.Event

	message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	if err = c.Do(message, func(e stun.Event) { res = e }); err != nil {
		return nil, err
	}
	if err = c.Close(); err != nil {
		return nil, err
	}

	if res.Error != nil {
		return nil, res.Error
	}

	var xorAddr stun.XORMappedAddress
	if err = xorAddr.GetFrom(res.Message); err != nil {
		return nil, err
	}

	return xorAddr.IP, nil
}

func IsIP(host string) bool {
	for _, i := range host {
		if i >= 'A' {
			return false
		}
	}
	return true
}

func MimeType(codec *streamer.Codec) string {
	switch codec.Name {
	case streamer.CodecH264:
		return webrtc.MimeTypeH264
	case streamer.CodecH265:
		return webrtc.MimeTypeH265
	case streamer.CodecVP8:
		return webrtc.MimeTypeVP8
	case streamer.CodecVP9:
		return webrtc.MimeTypeVP9
	case streamer.CodecAV1:
		return webrtc.MimeTypeAV1
	case streamer.CodecPCMU:
		return webrtc.MimeTypePCMU
	case streamer.CodecPCMA:
		return webrtc.MimeTypePCMA
	case streamer.CodecOpus:
		return webrtc.MimeTypeOpus
	case streamer.CodecG722:
		return webrtc.MimeTypeG722
	}
	panic("not implemented")
}
