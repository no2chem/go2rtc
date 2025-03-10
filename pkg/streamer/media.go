package streamer

import (
	"fmt"
	"github.com/pion/sdp/v3"
	"strconv"
	"strings"
	"unicode"
)

const (
	DirectionRecvonly = "recvonly"
	DirectionSendonly = "sendonly"
	DirectionSendRecv = "sendrecv"
)

const (
	KindVideo = "video"
	KindAudio = "audio"
)

const (
	CodecH264 = "H264" // payloadType: 96
	CodecH265 = "H265"
	CodecVP8  = "VP8"
	CodecVP9  = "VP9"
	CodecAV1  = "AV1"
	CodecJPEG = "JPEG" // payloadType: 26

	CodecPCMU = "PCMU" // payloadType: 0
	CodecPCMA = "PCMA" // payloadType: 8
	CodecAAC  = "MPEG4-GENERIC"
	CodecOpus = "OPUS" // payloadType: 111
	CodecG722 = "G722"
	CodecMPA  = "MPA" // payload: 14
)

func GetKind(name string) string {
	switch name {
	case CodecH264, CodecH265, CodecVP8, CodecVP9, CodecAV1, CodecJPEG:
		return KindVideo
	case CodecPCMU, CodecPCMA, CodecAAC, CodecOpus, CodecG722, CodecMPA:
		return KindAudio
	}
	return ""
}

// Media take best from:
// - deepch/vdk/format/rtsp/sdp.Media
// - pion/sdp.MediaDescription
type Media struct {
	Kind      string   `json:"kind,omitempty"` // video or audio
	Direction string   `json:"direction,omitempty"`
	Codecs    []*Codec `json:"codecs,omitempty"`

	MID     string `json:"mid,omitempty"`     // TODO: fixme?
	Control string `json:"control,omitempty"` // TODO: fixme?
	Title   string `json:"title,omitempty"`   // TODO: fixme?
}

func (m *Media) String() string {
	s := fmt.Sprintf("%s, %s", m.Kind, m.Direction)
	for _, codec := range m.Codecs {
		s += ", " + codec.String()
	}
	return s
}

func (m *Media) Clone() *Media {
	clone := *m
	return &clone
}

func (m *Media) AV() bool {
	return m.Kind == KindVideo || m.Kind == KindAudio
}

func (m *Media) MatchCodec(codec *Codec) bool {
	for _, c := range m.Codecs {
		if c.Match(codec) {
			return true
		}
	}
	return false
}

func (m *Media) MatchMedia(media *Media) *Codec {
	if m.Kind != media.Kind {
		return nil
	}

	switch m.Direction {
	case DirectionSendonly:
		if media.Direction != DirectionRecvonly {
			return nil
		}
	case DirectionRecvonly:
		if media.Direction != DirectionSendonly {
			return nil
		}
	default:
		panic("wrong direction")
	}

	for _, localCodec := range m.Codecs {
		if media.Codecs == nil {
			return localCodec
		}

		for _, remoteCodec := range media.Codecs {
			if localCodec.Match(remoteCodec) {
				return localCodec
			}
		}
	}
	return nil
}

// Codec take best from:
// - deepch/vdk/av.CodecData
// - pion/webrtc.RTPCodecCapability
type Codec struct {
	Name        string // H264, PCMU, PCMA, opus...
	ClockRate   uint32 // 90000, 8000, 16000...
	Channels    uint16 // 0, 1, 2
	FmtpLine    string
	PayloadType uint8
}

func NewCodec(name string) *Codec {
	name = strings.ToUpper(name)
	switch name {
	case CodecH264, CodecH265, CodecVP8, CodecVP9, CodecAV1, CodecJPEG:
		return &Codec{Name: name, ClockRate: 90000}
	case CodecPCMU, CodecPCMA:
		return &Codec{Name: name, ClockRate: 8000}
	case CodecOpus:
		return &Codec{Name: name, ClockRate: 48000, Channels: 2}
	case "MJPEG":
		return &Codec{Name: CodecJPEG, ClockRate: 90000}
	}

	panic(fmt.Sprintf("unsupported codec: %s", name))
}

func (c *Codec) String() string {
	s := fmt.Sprintf("%d %s/%d", c.PayloadType, c.Name, c.ClockRate)
	if c.Channels > 0 {
		s = fmt.Sprintf("%s/%d", s, c.Channels)
	}
	return s
}

func (c *Codec) Clone() *Codec {
	clone := *c
	return &clone
}

func (c *Codec) Match(codec *Codec) bool {
	return c.Name == codec.Name &&
		(c.ClockRate == codec.ClockRate || codec.ClockRate == 0) &&
		(c.Channels == codec.Channels || codec.Channels == 0)
}

func UnmarshalSDP(rawSDP []byte) ([]*Media, error) {
	sd := &sdp.SessionDescription{}
	if err := sd.Unmarshal(rawSDP); err != nil {
		return nil, err
	}

	var medias []*Media
	for _, md := range sd.MediaDescriptions {
		media := UnmarshalMedia(md)

		if media.Direction == DirectionSendRecv {
			media.Direction = DirectionRecvonly
			medias = append(medias, media)

			media = media.Clone()
			media.Direction = DirectionSendonly
		}

		medias = append(medias, media)
	}

	return medias, nil
}

func MarshalSDP(medias []*Media) ([]byte, error) {
	sd := &sdp.SessionDescription{}

	payloadType := uint8(96)

	for _, media := range medias {
		if media.Codecs == nil {
			continue
		}

		codec := media.Codecs[0]
		md := &sdp.MediaDescription{
			MediaName: sdp.MediaName{
				Media:  media.Kind,
				Protos: []string{"RTP", "AVP"},
			},
		}
		md.WithCodec(payloadType, codec.Name, codec.ClockRate, codec.Channels, codec.FmtpLine)

		sd.MediaDescriptions = append(sd.MediaDescriptions, md)

		payloadType++
	}

	return sd.Marshal()
}

func UnmarshalMedia(md *sdp.MediaDescription) *Media {
	m := &Media{
		Kind: md.MediaName.Media,
	}

	for _, attr := range md.Attributes {
		switch attr.Key {
		case DirectionSendonly, DirectionRecvonly, DirectionSendRecv:
			m.Direction = attr.Key
		case "control":
			m.Control = attr.Value
		case "mid":
			m.MID = attr.Value
		}
	}

	for _, format := range md.MediaName.Formats {
		m.Codecs = append(m.Codecs, UnmarshalCodec(md, format))
	}

	return m
}

func UnmarshalCodec(md *sdp.MediaDescription, payloadType string) *Codec {
	c := &Codec{PayloadType: byte(atoi(payloadType))}

	for _, attr := range md.Attributes {
		switch {
		case c.Name == "" && attr.Key == "rtpmap" && strings.HasPrefix(attr.Value, payloadType):
			i := strings.IndexByte(attr.Value, ' ')
			ss := strings.Split(attr.Value[i+1:], "/")

			c.Name = strings.ToUpper(ss[0])
			// fix tailing space: `a=rtpmap:96 H264/90000 `
			c.ClockRate = uint32(atoi(strings.TrimRightFunc(ss[1], unicode.IsSpace)))

			if len(ss) == 3 && ss[2] == "2" {
				c.Channels = 2
			}
		case c.FmtpLine == "" && attr.Key == "fmtp" && strings.HasPrefix(attr.Value, payloadType):
			if i := strings.IndexByte(attr.Value, ' '); i > 0 {
				c.FmtpLine = attr.Value[i+1:]
			}
		}
	}

	if c.Name == "" {
		// https://en.wikipedia.org/wiki/RTP_payload_formats
		switch payloadType {
		case "0":
			c.Name = CodecPCMU
			c.ClockRate = 8000
		case "8":
			c.Name = CodecPCMA
			c.ClockRate = 8000
		case "14":
			c.Name = CodecMPA
			c.ClockRate = 44100
		case "26":
			c.Name = CodecJPEG
			c.ClockRate = 90000
		default:
			c.Name = payloadType
		}
	}

	return c
}

func atoi(s string) (i int) {
	i, _ = strconv.Atoi(s)
	return
}
