package main

import (
	"bytes"
	"github.com/hajimehoshi/go-mp3"
	"github.com/hajimehoshi/oto/v2"
	"io"
	"log"
	"sync"
	"time"
)

// SoundEffect is an enum for different sound types.
type SoundEffect int

const (
	SoundCardPlay SoundEffect = iota
	SoundCapture
	SoundPisti
	SoundGameStart
	SoundPlayerWins
	SoundCPUWins
	SoundTie
	SoundBackground
	SoundUndo
	SoundDeal
	SoundPistiJack
)

var (
	otoCtx           *oto.Context
	soundData        = make(map[SoundEffect][]byte)
	lastPlayTimes    = make(map[SoundEffect]time.Time) // Per-sound rate limiting.
	soundLoaded      = false
	soundMutex       sync.Mutex                  // Protects the lastPlayTimes map and activePlayers.
	soundRateLimit   = 10 * time.Millisecond     // 10ms delay between sounds (allows faster playback).
	activePlayers    = make(map[oto.Player]bool) // Track active players for cleanup.
	backgroundPlayer oto.Player
)

// initAudio initializes the audio context. This must be called once at startup.
func initAudio() {
	// 44100, 2 channels (stereo), 2 bytes (16-bit) is a standard setting.
	var readyChan chan struct{}
	var err error
	otoCtx, readyChan, err = oto.NewContext(44100, 2, 2)
	if err != nil {
		log.Printf("ERROR: Failed to initialize audio context: %v. Audio will be disabled.", err)
		return
	}
	// The audio context needs a moment to initialize. Must wait for the ready signal before using it.
	// This is done in a separate goroutine to avoid blocking the UI from appearing.
	go func() {
		<-readyChan
		soundLoaded = true
		// Now that the context is ready, load the sounds.
		loadAllSounds()
		// Start a background goroutine to clean up finished audio players.
		go cleanupActivePlayers()
		// Once sounds are loaded, start the background music automatically.
		PlayBackgroundMusic()
	}()
}

// cleanupActivePlayers runs in the background and periodically removes finished
// sound effect players from the activePlayers map to prevent memory leaks.
func cleanupActivePlayers() {
	ticker := time.NewTicker(500 * time.Millisecond) // Check every half second.
	defer ticker.Stop()
	for range ticker.C {
		soundMutex.Lock()
		for player, active := range activePlayers {
			if active && !player.IsPlaying() {
				player.Close()
				delete(activePlayers, player)
			}
		}
		soundMutex.Unlock()
	}
}

// loadAllSounds is called once the audio context is ready.
func loadAllSounds() {
	loadSound(SoundCardPlay, "assets/sounds/play.mp3")
	loadSound(SoundCapture, "assets/sounds/capture.mp3")
	loadSound(SoundPistiJack, "assets/sounds/pisti_jack.mp3")
	loadSound(SoundPisti, "assets/sounds/pisti.mp3")
	loadSound(SoundGameStart, "assets/sounds/start.mp3")
	loadSound(SoundPlayerWins, "assets/sounds/player_wins.mp3")
	loadSound(SoundCPUWins, "assets/sounds/cpu_wins.mp3")
	loadSound(SoundTie, "assets/sounds/tie.mp3")
	loadSound(SoundBackground, "assets/sounds/background.mp3")
	loadSound(SoundUndo, "assets/sounds/undo.mp3")
	loadSound(SoundDeal, "assets/sounds/deal.mp3")
}

// loadSound loads a sound from the embedded assets into memory.
func loadSound(effect SoundEffect, path string) {
	if !soundLoaded {
		return // Audio context failed to initialize.
	}
	fileBytes, err := embeddedAssets.ReadFile(path)
	if err != nil {
		log.Printf("ERROR: Failed to load sound asset %s: %v", path, err)
		return
	}
	// Decode the entire mp3 file into a raw byte slice.
	decoder, err := mp3.NewDecoder(bytes.NewReader(fileBytes))
	if err != nil {
		log.Printf("ERROR: Failed to decode mp3 %s: %v", path, err)
		return
	}
	decodedBytes, err := io.ReadAll(decoder)
	if err != nil {
		log.Printf("ERROR: Failed to read decoded mp3 %s: %v", path, err)
		return
	}
	soundData[effect] = decodedBytes
}

// loopingReader is a custom io.Reader that wraps another reader and seeks
// to the beginning when it encounters an io.EOF, creating an infinite loop.
type loopingReader struct {
	reader io.ReadSeeker
}

// Read implements the io.Reader interface for looping playback.
func (lr *loopingReader) Read(p []byte) (n int, err error) {
	n, err = lr.reader.Read(p)
	if err == io.EOF {
		// When the end is reached, seek back to the start.
		if _, seekErr := lr.reader.Seek(0, io.SeekStart); seekErr != nil {
			return 0, seekErr // Return error if seek fails.
		}
		// The error is now nil as EOF has been handled by looping.
		err = nil
	}
	return n, err
}

// PlayBackgroundMusic starts the looping background music.
func PlayBackgroundMusic() {
	if !soundLoaded {
		return
	}
	// If the player is already created and playing, do nothing.
	if backgroundPlayer != nil && backgroundPlayer.IsPlaying() {
		return
	}
	data, ok := soundData[SoundBackground]
	if !ok || len(data) == 0 {
		return // Background music not loaded.
	}
	// Create an infinite loop stream from decoded music data.
	loopingStream := &loopingReader{reader: bytes.NewReader(data)}
	backgroundPlayer = otoCtx.NewPlayer(loopingStream)
	// Set the volume to 15% to ensure it's not overpowering.
	backgroundPlayer.SetVolume(0.15)
	backgroundPlayer.Play()
}

// PlaySound plays a pre-loaded sound effect.
func PlaySound(effect SoundEffect) {
	if !soundLoaded {
		return // Audio disabled.
	}
	// The rate limiter needs to be protected by a mutex to prevent race conditions
	// when sounds are triggered from different threads (e.g., UI and timers).
	soundMutex.Lock()
	// Rate limit each sound effect type individually.
	if time.Since(lastPlayTimes[effect]) < soundRateLimit {
		soundMutex.Unlock()
		return
	}
	lastPlayTimes[effect] = time.Now()
	data, ok := soundData[effect]
	if !ok || len(data) == 0 {
		soundMutex.Unlock()
		return // Sound not loaded.
	}
	// Create a new player for the sound effect.
	player := otoCtx.NewPlayer(bytes.NewReader(data))
	// Add it to the activePlayers map to prevent it from being garbage-collected
	// while it is playing. The cleanup goroutine will remove it later.
	activePlayers[player] = true
	soundMutex.Unlock()
	player.Play()
}
