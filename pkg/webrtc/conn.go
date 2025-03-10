package webrtc

import (
	"fmt"
	"github.com/AlexxIT/go2rtc/pkg/streamer"
	"github.com/pion/webrtc/v3"
)

const (
	MsgTypeOffer         = "webrtc/offer"
	MsgTypeOfferComplete = "webrtc/offer-complete"
	MsgTypeAnswer        = "webrtc/answer"
	MsgTypeCandidate     = "webrtc/candidate"
)

type Conn struct {
	streamer.Element

	UserAgent string

	Conn *webrtc.PeerConnection

	medias []*streamer.Media
	tracks []*streamer.Track

	receive int
	send    int
}

func (c *Conn) Init() {
	c.Conn.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			c.Fire(&streamer.Message{
				Type: MsgTypeCandidate, Value: candidate.ToJSON().Candidate,
			})
		}
	})

	c.Conn.OnTrack(func(remote *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		for _, track := range c.tracks {
			if track.Direction != streamer.DirectionRecvonly {
				continue
			}
			if track.Codec.PayloadType != uint8(remote.PayloadType()) {
				continue
			}

			for {
				packet, _, err := remote.ReadRTP()
				if err != nil {
					return
				}
				if len(packet.Payload) == 0 {
					continue
				}
				c.receive += len(packet.Payload)
				_ = track.WriteRTP(packet)
			}
		}

		fmt.Printf("TODO: webrtc ontrack %+v\n", remote)
	})

	// OK connection:
	// 15:01:46 ICE connection state changed: checking
	// 15:01:46 peer connection state changed: connected
	// 15:01:54 peer connection state changed: disconnected
	// 15:02:20 peer connection state changed: failed
	//
	// Fail connection:
	// 14:53:08 ICE connection state changed: checking
	// 14:53:39 peer connection state changed: failed
	c.Conn.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		c.Fire(state)

		// TODO: remove
		switch state {
		case webrtc.PeerConnectionStateConnected:
			c.Fire(streamer.StatePlaying) // TODO: remove
		case webrtc.PeerConnectionStateDisconnected:
			c.Fire(streamer.StateNull) // TODO: remove
			// disconnect event comes earlier, than failed
			// but it comes only for success connections
			_ = c.Conn.Close()
			c.Conn = nil
		case webrtc.PeerConnectionStateFailed:
			if c.Conn != nil {
				_ = c.Conn.Close()
			}
		}
	})
}

func (c *Conn) SetOffer(offer string) (err error) {
	sdOffer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer, SDP: offer,
	}
	if err = c.Conn.SetRemoteDescription(sdOffer); err != nil {
		return
	}
	rawSDP := []byte(c.Conn.RemoteDescription().SDP)
	c.medias, err = streamer.UnmarshalSDP(rawSDP)
	return
}

func (c *Conn) GetAnswer() (answer string, err error) {
	for _, tr := range c.Conn.GetTransceivers() {
		if tr.Direction() != webrtc.RTPTransceiverDirectionSendonly {
			continue
		}

		// disable transceivers if we don't have track
		// make direction=inactive
		// don't really necessary, but anyway
		if tr.Sender() == nil {
			if err = tr.Stop(); err != nil {
				return
			}
		}
	}

	var sdAnswer webrtc.SessionDescription
	sdAnswer, err = c.Conn.CreateAnswer(nil)
	if err != nil {
		return
	}

	if err = c.Conn.SetLocalDescription(sdAnswer); err != nil {
		return
	}

	return sdAnswer.SDP, nil
}

func (c *Conn) GetCompleteAnswer() (answer string, err error) {
	if _, err = c.GetAnswer(); err != nil {
		return
	}

	<-webrtc.GatheringCompletePromise(c.Conn)
	return c.Conn.LocalDescription().SDP, nil
}

func (c *Conn) remote() string {
	if c.Conn == nil {
		return ""
	}

	for _, trans := range c.Conn.GetTransceivers() {
		if trans == nil {
			continue
		}

		receiver := trans.Receiver()
		if receiver == nil {
			continue
		}

		transport := receiver.Transport()
		if transport == nil {
			continue
		}

		iceTransport := transport.ICETransport()
		if iceTransport == nil {
			continue
		}

		pair, _ := iceTransport.GetSelectedCandidatePair()
		if pair == nil || pair.Remote == nil {
			continue
		}

		return pair.Remote.String()
	}

	return ""
}
