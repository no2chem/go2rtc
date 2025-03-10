package api

import (
	"encoding/json"
	"github.com/AlexxIT/go2rtc/cmd/app"
	"github.com/AlexxIT/go2rtc/cmd/streams"
	"github.com/AlexxIT/go2rtc/pkg/streamer"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"net"
	"net/http"
)

func Init() {
	var cfg struct {
		Mod struct {
			Listen    string `yaml:"listen"`
			BasePath  string `yaml:"base_path"`
			StaticDir string `yaml:"static_dir"`
		} `yaml:"api"`
	}

	// default config
	cfg.Mod.Listen = ":1984"

	// load config from YAML
	app.LoadConfig(&cfg)

	if cfg.Mod.Listen == "" {
		return
	}

	basePath = cfg.Mod.BasePath
	log = app.GetLogger("api")

	initStatic(cfg.Mod.StaticDir)
	initWS()

	HandleFunc("api/streams", streamsHandler)
	HandleFunc("api/ws", apiWS)

	// ensure we can listen without errors
	listener, err := net.Listen("tcp", cfg.Mod.Listen)
	if err != nil {
		log.Fatal().Err(err).Msg("[api] listen")
		return
	}

	log.Info().Str("addr", cfg.Mod.Listen).Msg("[api] listen")

	go func() {
		s := http.Server{}

		if log.Trace().Enabled() {
			s.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				log.Trace().Stringer("url", r.URL).Msgf("[api] %s", r.Method)
				http.DefaultServeMux.ServeHTTP(w, r)
			})
		}

		if err = s.Serve(listener); err != nil {
			log.Fatal().Err(err).Msg("[api] serve")
		}
	}()
}

// HandleFunc handle pattern with relative path:
// - "api/streams" => "{basepath}/api/streams"
// - "/streams"    => "/streams"
func HandleFunc(pattern string, handler http.HandlerFunc) {
	if len(pattern) == 0 || pattern[0] != '/' {
		pattern = basePath + "/" + pattern
	}
	log.Trace().Str("path", pattern).Msg("[api] register path")
	http.HandleFunc(pattern, handler)
}

func HandleWS(msgType string, handler WSHandler) {
	wsHandlers[msgType] = handler
}

var basePath string
var log zerolog.Logger
var wsHandlers = make(map[string]WSHandler)

func streamsHandler(w http.ResponseWriter, r *http.Request) {
	src := r.URL.Query().Get("src")

	switch r.Method {
	case "PUT":
		streams.New(src, src)
		return
	case "DELETE":
		streams.Delete(src)
		return
	}

	var v interface{}
	if src != "" {
		v = streams.Get(src)
	} else {
		v = streams.All()
	}

	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	_ = e.Encode(v)
}

func apiWS(w http.ResponseWriter, r *http.Request) {
	ctx := new(Context)
	if err := ctx.Upgrade(w, r); err != nil {
		log.Error().Err(err).Msg("[api.ws] upgrade")
		return
	}
	defer ctx.Close()

	for {
		msg := new(streamer.Message)
		if err := ctx.Conn.ReadJSON(msg); err != nil {
			if websocket.IsUnexpectedCloseError(
				err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure,
			) {
				log.Error().Err(err).Msg("[api.ws] readJSON")
			}
			return
		}

		handler := wsHandlers[msg.Type]
		if handler != nil {
			handler(ctx, msg)
		}
	}
}
