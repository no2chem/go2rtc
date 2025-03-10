package rtmp

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/AlexxIT/go2rtc/pkg/h264"
	"github.com/AlexxIT/go2rtc/pkg/streamer"
	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/codec/aacparser"
	"github.com/deepch/vdk/codec/h264parser"
	"github.com/deepch/vdk/format/rtmp"
	"github.com/pion/rtp"
	"time"
)

type Client struct {
	streamer.Element

	URI string

	medias []*streamer.Media
	tracks []*streamer.Track

	conn   *rtmp.Conn
	closed bool

	receive int
}

func NewClient(uri string) *Client {
	return &Client{URI: uri}
}

func (c *Client) Dial() (err error) {
	c.conn, err = rtmp.Dial(c.URI)
	if err != nil {
		return
	}

	// important to get SPS/PPS
	streams, err := c.conn.Streams()
	if err != nil {
		return
	}

	for _, stream := range streams {
		switch stream.Type() {
		case av.H264:
			cd := stream.(h264parser.CodecData)
			fmtp := "sprop-parameter-sets=" +
				base64.StdEncoding.EncodeToString(cd.RecordInfo.SPS[0]) + "," +
				base64.StdEncoding.EncodeToString(cd.RecordInfo.PPS[0])

			codec := &streamer.Codec{
				Name:        streamer.CodecH264,
				ClockRate:   90000,
				FmtpLine:    fmtp,
				PayloadType: h264.PayloadTypeAVC,
			}

			media := &streamer.Media{
				Kind:      streamer.KindVideo,
				Direction: streamer.DirectionSendonly,
				Codecs:    []*streamer.Codec{codec},
			}
			c.medias = append(c.medias, media)

			track := &streamer.Track{
				Codec: codec, Direction: media.Direction,
			}
			c.tracks = append(c.tracks, track)

		case av.AAC:
			// TODO: fix support
			cd := stream.(aacparser.CodecData)

			// a=fmtp:97 streamtype=5;profile-level-id=1;mode=AAC-hbr;sizelength=13;indexlength=3;indexdeltalength=3;config=1588
			fmtp := fmt.Sprintf(
				"config=%s",
				hex.EncodeToString(cd.ConfigBytes),
			)

			codec := &streamer.Codec{
				Name:      streamer.CodecAAC,
				ClockRate: uint32(cd.Config.SampleRate),
				Channels:  uint16(cd.Config.ChannelConfig),
				FmtpLine:  fmtp,
			}

			media := &streamer.Media{
				Kind:      streamer.KindAudio,
				Direction: streamer.DirectionSendonly,
				Codecs:    []*streamer.Codec{codec},
			}
			c.medias = append(c.medias, media)

			track := &streamer.Track{
				Codec: codec, Direction: media.Direction,
			}
			c.tracks = append(c.tracks, track)

		default:
			fmt.Printf("[rtmp] unsupported codec %+v\n", stream)
		}
	}

	c.Fire(streamer.StateReady)

	return
}

func (c *Client) Handle() (err error) {
	defer c.Fire(streamer.StateNull)

	c.Fire(streamer.StatePlaying)

	for {
		var pkt av.Packet
		pkt, err = c.conn.ReadPacket()
		if err != nil {
			if c.closed {
				return nil
			}
			return
		}

		c.receive += len(pkt.Data)

		track := c.tracks[int(pkt.Idx)]

		timestamp := uint32(pkt.Time / time.Duration(track.Codec.ClockRate))

		var payloads [][]byte
		if track.Codec.Name == streamer.CodecH264 {
			payloads = h264.SplitAVC(pkt.Data)
		} else {
			payloads = [][]byte{pkt.Data}
		}

		for _, payload := range payloads {
			packet := &rtp.Packet{
				Header:  rtp.Header{Timestamp: timestamp},
				Payload: payload,
			}
			_ = track.WriteRTP(packet)
		}
	}
}

func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	c.closed = true
	return c.conn.Close()
}
