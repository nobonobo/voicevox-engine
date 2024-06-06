package engine

import (
	"context"
	"io"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	gcache "github.com/Code-Hex/go-generics-cache"
	"github.com/aethiopicuschan/nanoda"
	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

var ptn = regexp.MustCompile(`[&,.;、。・]`)

func Clean(s string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, s))
}

type Engine struct {
	ctx    context.Context
	vox    *nanoda.Voicevox
	synth  nanoda.Synthesizer
	queue  chan io.Reader
	cache  *gcache.Cache[string, []byte]
	input  chan string
	reader io.Reader
	writer io.Writer
	abort  uint32
	cancel func()
}

func New(ctx context.Context) *Engine {
	reader, writer := io.Pipe()
	return &Engine{
		ctx:    ctx,
		queue:  make(chan io.Reader, 1),
		input:  make(chan string, 1),
		reader: reader,
		writer: writer,
		cache:  gcache.NewContext[string, []byte](ctx),
		cancel: func() {},
	}
}

func (e *Engine) Synthesis(text string) ([]byte, error) {
	cached, ok := e.cache.Get(text)
	if ok {
		return cached, nil
	}
	q, err := e.synth.CreateAudioQuery(text, nanoda.StyleId(Config.ActorID))
	if err != nil {
		return nil, err
	}
	q.IntonationScale = Config.Intnation
	q.PitchScale = Config.Pitch
	q.SpeedScale = Config.Speed
	q.VolumeScale = Config.Volume
	q.PrePhonemeLength = Config.PrePhonemeLength
	q.PostPhonemeLength = Config.PostPhonemeLength
	for _, p := range q.AccentPhrases[1:] {
		if p.PauseMora != nil {
			p.PauseMora.VowelLength *= Config.Pause
		}
	}
	w, err := e.synth.Synthesis(q, nanoda.StyleId(Config.ActorID))
	if err != nil {
		return nil, err
	}
	defer w.Close()
	decoded, err := wav.DecodeWithoutResampling(w)
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(decoded)
	if err != nil {
		return nil, err
	}
	e.cache.Set(text, b, gcache.WithExpiration(time.Hour))
	return b, nil
}

func (e *Engine) Abort() {
	atomic.StoreUint32(&e.abort, 1)
}
func (e *Engine) IsAbort() bool {
	return atomic.LoadUint32(&e.abort) > 0
}

func (e *Engine) Play(text string) {
	atomic.StoreUint32(&e.abort, 0)
	words := strings.Fields(ptn.ReplaceAllString(text, " "))
	for _, word := range words {
		if e.IsAbort() {
			return
		}
		b, err := e.Synthesis(word)
		if err != nil {
			log.Println(err)
			continue
		}
		e.writer.Write(b)
	}
}

func (e *Engine) Run() error {
	ctx, cancel := context.WithCancel(e.ctx)
	e.cancel = cancel
	v, err := nanoda.NewVoicevox(
		filepath.Join(Config.VoiceVoxDir, "voicevox_core.dll"),
		filepath.Join(Config.VoiceVoxDir, "open_jtalk_dic_utf_8-1.11"),
		filepath.Join(Config.VoiceVoxDir, "model"))
	if err != nil {
		return err
	}
	e.vox = v
	s, err := v.NewSynthesizer()
	if err != nil {
		return err
	}
	e.synth = s
	if err := e.synth.LoadModelsFromStyleId(nanoda.StyleId(Config.ActorID)); err != nil {
		return err
	}
	otoCtx, wait, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   48000,
		ChannelCount: 1,
		Format:       oto.FormatSignedInt16LE,
	})
	if err != nil {
		return err
	}
	<-wait
	player := otoCtx.NewPlayer(e.reader)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				player.Play()
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return nil
		case line := <-e.input:
			e.Play(line)
		}
	}
}

func (e *Engine) Stop() error {
	e.synth.Close()
	e.cancel()
	return nil
}

func (e *Engine) Speak(text string) {
	text = Clean(text)
	if len(text) == 0 {
		return
	}
	e.Abort()
	e.input <- text
}
