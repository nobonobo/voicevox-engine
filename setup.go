package engine

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/caarlos0/env/v10"
	"golang.org/x/sys/windows"
)

const (
	downloadUrl = "https://github.com/VOICEVOX/voicevox_core/releases/download/0.15.0-preview.13/download-windows-x64.exe"
)

type Conf struct {
	VoiceVoxDir       string
	ActorID           int     `env:"VOICEVOX_ACTOR_ID" envDefault:"3"`
	Pitch             float64 `env:"VOICEVOX_PITCH" envDefault:"0.0"`
	Intnation         float64 `env:"VOICEVOX_INTNATION" envDefault:"1.0"`
	Speed             float64 `env:"VOICEVOX_SPEED" envDefault:"1.5"`
	Volume            float64 `env:"VOICEVOX_VOLUME" envDefault:"1.8"`
	Pause             float64 `env:"VOICEVOX_PAUSE" envDefault:"0.1"`
	PrePhonemeLength  float64 `env:"VOICEVOX_PRE_PHONEME_LEN" envDefault:"0.0"`
	PostPhonemeLength float64 `env:"VOICEVOX_POST_PHONEME_LEN" envDefault:"0.1"`
}

var Config = Conf{}

func download(u, folder string) error {
	info, err := url.Parse(u)
	if err != nil {
		return err
	}
	fname := filepath.Join(folder, filepath.Base(info.Path))
	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	f, err := os.Create(fname)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return nil
}

func isInstalled(folder string) bool {
	files := []string{
		"voicevox_core.dll",
		"onnxruntime_providers_shared.dll",
		"onnxruntime.dll",
		"open_jtalk_dic_utf_8-1.11",
		"model",
	}
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(folder, f)); err != nil {
			return false
		}
	}
	return true
}

func installVoiceVox(folder string) error {
	if err := download(downloadUrl, folder); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "./download-windows-x64.exe",
		"--device", "cpu", "--version", "0.15.0-preview.13",
	)
	cmd.Dir = folder
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	log.Println("install voicevox_core:", folder)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func init() {
	if err := env.Parse(&Config); err != nil {
		log.Fatal(err)
	}
}

func Setup(root string) {
	Config.VoiceVoxDir = filepath.Join(root, "voicevox_core")
	if err := windows.SetDllDirectory(Config.VoiceVoxDir); err != nil {
		log.Fatal(err)
	}
	if isInstalled(Config.VoiceVoxDir) {
		return
	}
	if err := os.RemoveAll(Config.VoiceVoxDir); err != nil {
		log.Fatal(err)
	}
	if err := installVoiceVox(filepath.Dir(Config.VoiceVoxDir)); err != nil {
		log.Fatal(err)
	}
}
