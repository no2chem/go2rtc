package srtp

import (
	"encoding/binary"
	"net"
)

// Server using same UDP port for SRTP and for SRTCP as the iPhone does
// this is not really necessary but anyway
type Server struct {
	sessions map[uint32]*Session
}

func (s *Server) AddSession(session *Session) {
	if s.sessions == nil {
		s.sessions = map[uint32]*Session{}
	}
	s.sessions[session.RemoteSSRC] = session
}

func (s *Server) RemoveSession(session *Session) {
	delete(s.sessions, session.RemoteSSRC)
}

func (s *Server) Serve(conn net.PacketConn) error {
	buf := make([]byte, 2048)
	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			return err
		}

		// Multiplexing RTP Data and Control Packets on a Single Port
		// https://datatracker.ietf.org/doc/html/rfc5761

		// this is default position for SSRC in RTP packet
		ssrc := binary.BigEndian.Uint32(buf[8:])
		session, ok := s.sessions[ssrc]
		if ok {
			if session.Write == nil {
				session.Write = func(b []byte) (int, error) {
					return conn.WriteTo(b, addr)
				}
			}

			if err = session.HandleRTP(buf[:n]); err != nil {
				return err
			}
		} else {
			// this is default position for SSRC in RTCP packet
			ssrc = binary.BigEndian.Uint32(buf[4:])
			if session, ok = s.sessions[ssrc]; !ok {
				continue // skip unknown ssrc
			}

			if err = session.HandleRTCP(buf[:n]); err != nil {
				return err
			}
		}
	}
}
