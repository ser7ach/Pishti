package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"
)

// GameState defines the possible states of the game.
type GameState int

const (
	StateNotStarted GameState = iota
	StatePlayerTurn
	StateCPUTurn
	StateGameOver
	StatePileCaptured
)

// PlayerID identifies who is taking an action.
type PlayerID int

const (
	NoPlayer PlayerID = 0
	Player   PlayerID = 1
	CPU      PlayerID = 2
)

// GameLevel defines the difficulty levels.
type GameLevel int

const (
	LevelNotSelected GameLevel = iota
	LevelBeginner
	LevelIntermediate
	LevelAdvanced
)

const (
	DeckSize = 52
	HandSize = 4
)

// Casino represents the game logic and state.
// This struct will hold the game state, and methods will implement the game logic.
type Casino struct {
	cpuCards                   []*Card
	allPlayedCardsMemory       []*Card // Long-term memory for Advanced AI, tracking all cards played during the game.
	currentHandMemory          []*Card // Short-term memory for Intermediate/Advanced AI, tracking cards in the current hand.
	deck                       []*Card
	playerCards                []*Card
	tableCards                 []*Card
	faces                      []string
	suits                      []string
	cardsCollectedByPlayer     int // Total number of cards collected by the player.
	cardsCollectedByCPU        int // Total number of cards collected by the CPU.
	cardsOnTable               int // Number of cards currently on the table pile.
	currentHandMemoryLength    int // Number of cards in the current hand memory.
	allPlayedCardsMemoryLength int // Number of cards in the CPU's long-term memory.
	cpuPoint                   int // CPU's current score.
	currentCard                int // Index for dealing from the deck.
	gameState                  GameState
	initialHiddenCards         []*Card   // The three face-down cards at the start of the game.
	safeDiscardCandidate       *Card     // Card face that is likely safe to discard.
	initialPileCaptureMsg      string    // Message to show when the initial pile is captured.
	lastPlayedCPUCardIdx       int       // Index of the CPU card played.
	lastPlayedPlayerCard       int       // Index of the player card played.
	lastScorer                 PlayerID  // Tracks who made the last capture (Player or CPU).
	level                      GameLevel // The selected difficulty level.
	playerPoint                int       // Player's current score.
	canUndo                    bool
	isInitialPile              bool
	undoState                  UndoState
	rng                        *rand.Rand // Random number generator instance.
	mu                         sync.Mutex // Mutex to protect concurrent access to game state.
}

// UndoState holds a snapshot of the game state for the undo feature.
type UndoState struct {
	playerPoint             int
	cpuPoint                int
	lastScorer              PlayerID
	cardsCollectedByPlayer  int
	cardsCollectedByCPU     int
	cardsOnTable            int
	tableCards              []*Card
	playerCards             []*Card
	initialHiddenCards      []*Card
	safeDiscardCandidate    *Card
	isInitialPile           bool
	cpuCards                []*Card
	currentHandMemory       []*Card
	currentHandMemoryLength int
}

// NewCasino initializes a new game instance.
func NewCasino() *Casino {
	c := &Casino{
		faces:     []string{"Ace", "Deuce", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine", "Ten", "Jack", "Queen", "King"},
		suits:     []string{"Hearts", "Diamonds", "Clubs", "Spades"},
		gameState: StateNotStarted,
		level:     LevelNotSelected,
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())), // Initialize RNG once.
	}
	// Initialize card arrays/slices.
	c.cpuCards = make([]*Card, HandSize)
	c.allPlayedCardsMemory = make([]*Card, DeckSize) // Pre-allocate for all cards to avoid resizing during gameplay.
	c.currentHandMemory = make([]*Card, DeckSize)    // A hand can have up to 8 cards played.
	c.playerCards = make([]*Card, HandSize)
	c.tableCards = make([]*Card, DeckSize) // The table can theoretically hold all cards.
	c.deck = make([]*Card, DeckSize)
	for i := 0; i < DeckSize; i++ {
		c.deck[i] = NewCard(c.faces[i%13], c.suits[i/13], strconv.Itoa(i+1))
	}
	return c
}

// shuffle shuffles the deck of cards.
func (c *Casino) shuffle() {
	for i := 0; i < DeckSize; i++ {
		swapped := c.rng.Intn(DeckSize)
		c.deck[i], c.deck[swapped] = c.deck[swapped], c.deck[i]
	}
}

// deal deals 4 cards to the player and 4 to the CPU.
func (c *Casino) deal() {
	if c.currentCard >= DeckSize {
		// This should ideally not happen if checkEndOfHand correctly sets StateGameOver,
		// but as a safeguard, prevent out-of-bounds access if the deck is exhausted.
		return
	}
	// Only play the deal sound for subsequent hands, not the initial one.
	if !c.isInitialPile {
		PlaySound(SoundDeal)
		c.safeDiscardCandidate = nil // Reset the safe discard clue for the new hand.
		// Reset the short-term memory for the new hand.
		for i := 0; i < c.currentHandMemoryLength; i++ {
			c.currentHandMemory[i] = nil
		}
		c.currentHandMemoryLength = 0
	}
	for i := 0; i < HandSize; i++ {
		c.playerCards[i] = c.deck[c.currentCard]
		c.currentCard++
	}
	for i := 0; i < HandSize; i++ {
		c.cpuCards[i] = c.deck[c.currentCard]
		c.currentCard++
	}
}

// StartGame initializes a new game.
func (c *Casino) StartGame() bool {
	c.mu.Lock() // Lock the entire StartGame operation.
	defer c.mu.Unlock()
	// If a game is already in progress or finished, reset it first.
	if c.gameState != StateNotStarted {
		c.resetGameInternal()
	}
	if c.level == LevelNotSelected {
		return false // Cannot start without a level.
	}
	c.currentCard = 0 // Crucial: Ensure currentCard is reset before shuffle sets it.
	c.shuffle()
	// Reset scores and counters.
	c.playerPoint = 0
	c.cpuPoint = 0
	c.cardsCollectedByPlayer = 0
	c.safeDiscardCandidate = nil
	c.initialHiddenCards = nil
	c.cardsCollectedByCPU = 0
	c.cardsOnTable = HandSize // Initial 4 cards on the table.
	c.currentHandMemoryLength = 0
	c.allPlayedCardsMemoryLength = 0
	// Set initial game state.
	c.gameState = StatePlayerTurn // Game starts with the player's turn.
	c.isInitialPile = true        // This is the initial pile before any move is made.
	c.lastPlayedCPUCardIdx = -1
	c.lastPlayedPlayerCard = -1
	// Deal initial 4 cards to the table.
	for i := 0; i < HandSize; i++ {
		c.tableCards[i] = c.deck[i]
	}
	// Store the initial three "hidden" cards to be revealed later.
	if len(c.tableCards) >= 3 {
		c.initialHiddenCards = []*Card{c.tableCards[0], c.tableCards[1], c.tableCards[2]}
	}
	c.currentCard = HandSize // Advance the deck pointer past the 4 table cards.
	c.deal()                 // Deal player and CPU hands.
	return true
}

// ResetGame resets the game to its initial state without starting a new one.
func (c *Casino) ResetGame() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resetGameInternal()
}

// resetGameInternal clears all game-specific state to prepare for a new game.
// This is an internal helper and assumes the mutex is already held by the caller.
func (c *Casino) resetGameInternal() {
	c.gameState = StateNotStarted
	c.level = LevelNotSelected // Crucial: Reset the selected level.
	c.cardsCollectedByPlayer = 0
	c.cardsCollectedByCPU = 0
	c.cardsOnTable = 0
	c.currentHandMemoryLength = 0
	c.allPlayedCardsMemoryLength = 0
	// Clear the Advanced AI's long-term memory for a new game.
	for i := 0; i < len(c.allPlayedCardsMemory); i++ {
		c.allPlayedCardsMemory[i] = nil
	}
	c.cpuPoint = 0
	c.playerPoint = 0
	c.lastPlayedCPUCardIdx = -1
	c.lastPlayedPlayerCard = -1
	c.lastScorer = NoPlayer
	c.isInitialPile = false
	c.canUndo = false
	c.currentCard = 0 // Crucial: Reset deck pointer.
	c.safeDiscardCandidate = nil
	c.initialHiddenCards = nil
	c.initialPileCaptureMsg = ""
	// Clear all card slices.
	for i := 0; i < HandSize; i++ {
		c.playerCards[i] = nil
		c.cpuCards[i] = nil
	}
	for i := 0; i < DeckSize; i++ {
		c.tableCards[i] = nil
		c.currentHandMemory[i] = nil
		c.allPlayedCardsMemory[i] = nil
	}
}

// SetLevel sets the game difficulty level.
func (c *Casino) SetLevel(level GameLevel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if level >= LevelNotSelected && level <= LevelAdvanced {
		c.level = GameLevel(level)
	} else {
		fmt.Println("Invalid level selected.")
	}
}

// playerPlays handles just the player's turn.
func (c *Casino) playerPlays(playedCardIdx int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Only save state for undo if the level allows it.
	if c.level != LevelAdvanced {
		undoTableCards := make([]*Card, c.cardsOnTable)
		copy(undoTableCards, c.tableCards[:c.cardsOnTable])
		undoPlayerCards := make([]*Card, HandSize)
		copy(undoPlayerCards, c.playerCards)
		undoCPUCards := make([]*Card, HandSize)
		copy(undoCPUCards, c.cpuCards)
		// Also save the state of the current hand's memory.
		undoHandMemory := make([]*Card, c.currentHandMemoryLength)
		copy(undoHandMemory, c.currentHandMemory[:c.currentHandMemoryLength])
		c.undoState = UndoState{
			playerPoint: c.playerPoint, cpuPoint: c.cpuPoint, lastScorer: c.lastScorer,
			cardsCollectedByPlayer: c.cardsCollectedByPlayer, cardsCollectedByCPU: c.cardsCollectedByCPU,
			cardsOnTable: c.cardsOnTable, tableCards: undoTableCards,
			playerCards: undoPlayerCards, cpuCards: undoCPUCards,
		}
		c.undoState.isInitialPile = c.isInitialPile
		c.undoState.initialHiddenCards = c.initialHiddenCards
		c.undoState.safeDiscardCandidate = c.safeDiscardCandidate
		c.undoState.currentHandMemory = undoHandMemory
		c.undoState.currentHandMemoryLength = c.currentHandMemoryLength
	}
	playerPlayedCard := c.playerCards[playedCardIdx]
	if playerPlayedCard == nil {
		return
	}
	c.lastPlayedPlayerCard = playedCardIdx
	c.playerCards[playedCardIdx] = nil
	c.processTurn(playerPlayedCard, Player)
	// The first time a player plays a card, the initial pile state is over.
	if c.isInitialPile {
		c.isInitialPile = false
	}
}

// cpuPlays handles just the CPU's turn.
func (c *Casino) cpuPlays() {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Check if the CPU has any cards to play. If not, do nothing.
	// This prevents a crash at the end of a hand.
	hasCards := false
	for _, card := range c.cpuCards {
		if card != nil {
			hasCards = true
			break
		}
	}
	if !hasCards {
		return
	}
	c.lastPlayedCPUCardIdx = c.CPUaction()
	if c.lastPlayedCPUCardIdx == -1 {
		// This should never happen, but as a safeguard, find any valid card.
		for i := 0; i < HandSize; i++ {
			if c.cpuCards[i] != nil {
				c.lastPlayedCPUCardIdx = i
				break
			}
		}
	}
	cpuPlayedCard := c.cpuCards[c.lastPlayedCPUCardIdx]
	if cpuPlayedCard != nil {
		c.cpuCards[c.lastPlayedCPUCardIdx] = nil
		c.processTurn(cpuPlayedCard, CPU)
		c.canUndo = true
	}
}

// processTurn handles the logic for a single card play, for either the player or CPU.
func (c *Casino) processTurn(playedCard *Card, playerID PlayerID) {
	if playedCard == nil {
		fmt.Printf("ERROR: Player %d tried to play an empty card slot.\n", playerID)
		return
	}
	// Update AI memory based on the difficulty level.
	switch c.level {
	case LevelIntermediate:
		// Intermediate AI uses short-term memory for the current hand.
		if c.currentHandMemoryLength < len(c.currentHandMemory) {
			c.currentHandMemory[c.currentHandMemoryLength] = playedCard
			c.currentHandMemoryLength++
		}
	case LevelAdvanced:
		// Advanced AI uses long-term memory for the entire game.
		if c.allPlayedCardsMemoryLength < len(c.allPlayedCardsMemory) {
			c.allPlayedCardsMemory[c.allPlayedCardsMemoryLength] = playedCard
			c.allPlayedCardsMemoryLength++
		}
	}
	PlaySound(SoundCardPlay) // Play sound for every card played.
	c.tableCards[c.cardsOnTable] = playedCard
	c.cardsOnTable++
	// Check for scoring.
	if c.cardsOnTable > 1 {
		topCardOnTable := c.tableCards[c.cardsOnTable-1]
		secondToTopCard := c.tableCards[c.cardsOnTable-2]
		if topCardOnTable.GetFace() == secondToTopCard.GetFace() || topCardOnTable.GetFace() == "Jack" {
			// If player captures with a Jack, the card underneath is a safe discard candidate for the AI.
			if playerID == Player && topCardOnTable.GetFace() == "Jack" {
				c.safeDiscardCandidate = secondToTopCard
			}
			points := 0
			// If the initial hidden cards are still being tracked, this capture is for the initial pile.
			if c.initialHiddenCards != nil {
				// Only show the message if the player is the one capturing.
				if playerID == Player {
					c.initialPileCaptureMsg = fmt.Sprintf("You captured the hidden cards: %s, %s, and %s!",
						c.initialHiddenCards[0].GetFace(), c.initialHiddenCards[1].GetFace(), c.initialHiddenCards[2].GetFace())
				}
				c.initialHiddenCards = nil // The initial pile has been captured, so clear the tracker.
			}
			cardsCollected := 0
			if c.cardsOnTable == 2 && topCardOnTable.GetFace() == secondToTopCard.GetFace() {
				if topCardOnTable.GetFace() == "Jack" {
					points = 20 // Jack Pişti(House Rule).
					PlaySound(SoundPistiJack)
				} else {
					points = 10 // Standard Pişti.
					PlaySound(SoundPisti)
				}
				cardsCollected = 2
			} else {
				// Normal pile collection.
				points = c.pointCalculator()
				cardsCollected = c.cardsOnTable
				PlaySound(SoundCapture)
			}
			if playerID == Player {
				c.playerPoint += points
				c.cardsCollectedByPlayer += cardsCollected
			} else {
				c.cpuPoint += points
				c.cardsCollectedByCPU += cardsCollected
			}
			// Instead of clearing the table immediately, set a new state
			// to allow the UI to show the captured pile for a moment.
			c.gameState = StatePileCaptured
			c.lastScorer = playerID
			return // Return early to prevent gameState from being overwritten.
		}
	}
	// If no capture, set the turn to the other player.
	if playerID == Player {
		c.gameState = StateCPUTurn
	} else {
		c.gameState = StatePlayerTurn
	}
}

// finalizeCapture completes the capture process by clearing the table.
func (c *Casino) finalizeCapture() {
	// This function is called after the StatePileCaptured pause.
	// It clears the table and sets the turn to the correct player.
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cardsOnTable = 0
	// Do not clear the initialPileCaptureMsg here. It should persist until the player's next move.
	// If the game is over (e.g., last card captured the pile), do not
	// revert the state back to a player's turn.
	if c.gameState != StateGameOver {
		// The player who just scored gets to play again.
		c.gameState = StatePlayerTurn
	}
}

// isHandFinished is a read-only helper to check if the current hand is over.
func (c *Casino) isHandFinished() bool {
	// A hand is finished if the player has no cards left.
	// Since player and CPU play in tandem, we only need to check one.
	// This is an internal helper; the caller must hold the mutex.
	for _, card := range c.playerCards {
		if card != nil {
			return false
		}
	}
	return true
}

func (c *Casino) checkEndOfHand() {
	c.mu.Lock()
	defer c.mu.Unlock()
	// If the game is already over, don't re-process the end-of-game logic.
	// This is the primary guard against race conditions from multiple UI timers.
	if c.gameState == StateGameOver {
		return
	}
	// isHandFinished() checks if both players have run out of cards.
	if c.isHandFinished() {
		if c.currentCard == DeckSize {
			c.handleEndOfGame()
		} else {
			c.handleEndOfHand()
		}
	}
}

// handleEndOfHand is called when a hand is over but the deck is not empty.
// It deals a new hand and continues the game. Assumes caller holds the mutex.
func (c *Casino) handleEndOfHand() {
	c.deal()
	c.gameState = StatePlayerTurn // Resume play.
}

// handleEndOfGame is called when the final hand is played and the deck is empty.
// It finalizes scores and sets the game state to GameOver. Assumes caller holds the mutex.
func (c *Casino) handleEndOfGame() {
	c.awardFinalPile()
	c.gameState = StateGameOver
	// Award the card count bonus after the final pile is collected.
	// If it's a tie (26-26), no one gets points.
	if c.cardsCollectedByPlayer > c.cardsCollectedByCPU {
		c.playerPoint += 3
	} else if c.cardsCollectedByCPU > c.cardsCollectedByPlayer {
		c.cpuPoint += 3
	}
}

// awardFinalPile gives the remaining cards on the table to the last player who scored.
// This is an internal helper that assumes the caller holds the mutex.
func (c *Casino) awardFinalPile() {
	if c.cardsOnTable <= 0 {
		return // Nothing to award.
	}
	pointsFromLastPile := c.pointCalculator()
	if c.lastScorer == Player { // Player gets the last pile.
		c.playerPoint += pointsFromLastPile
		c.cardsCollectedByPlayer += c.cardsOnTable
	} else { // CPU or no scorer yet (CPU gets it by default).
		c.cpuPoint += pointsFromLastPile
		c.cardsCollectedByCPU += c.cardsOnTable
	}
	c.cardsOnTable = 0 // Reset the table card counter.
}

// findMatchingCard looks for a card with a specific face in the CPU's hand.
func (c *Casino) findMatchingCard(face string) int {
	for i, card := range c.cpuCards {
		if card != nil && card.GetFace() == face {
			return i
		}
	}
	return -1 // Not found.
}

// findJack looks for a Jack in the CPU's hand.
func (c *Casino) findJack() int {
	for i, card := range c.cpuCards {
		if card != nil && card.GetFace() == "Jack" {
			return i
		}
	}
	return -1 // Not found.
}

// findRandomNonJack finds a random card in the CPU's hand that is not a Jack.
func (c *Casino) findRandomNonJack() int {
	var cardsToPlay []int
	for i := 0; i < HandSize; i++ {
		if c.cpuCards[i] != nil && c.cpuCards[i].GetFace() != "Jack" {
			cardsToPlay = append(cardsToPlay, i)
		}
	}
	if len(cardsToPlay) > 0 {
		return cardsToPlay[c.rng.Intn(len(cardsToPlay))] // Use Casino's RNG.
	}
	return -1 // No non-jack found
}

// tryCaptureMove checks if the CPU can make a capturing move (match top card or play a Jack).
func (c *Casino) tryCaptureMove() int {
	if c.cardsOnTable > 0 {
		topCardFace := c.tableCards[c.cardsOnTable-1].GetFace()
		// Try to match the top card.
		if cardIdx := c.findMatchingCard(topCardFace); cardIdx != -1 {
			return cardIdx
		}
		// Try to play Jack.
		if cardIdx := c.findJack(); cardIdx != -1 {
			return cardIdx
		}
	}
	return -1 // No capture move found.
}

// trySafeDiscard checks if there's a known safe card to play (based on player's Jack capture).
func (c *Casino) trySafeDiscard() int {
	if c.safeDiscardCandidate != nil {
		if cardIdx := c.findMatchingCard(c.safeDiscardCandidate.GetFace()); cardIdx != -1 {
			return cardIdx
		}
	}
	return -1 // No safe discard move found.
}

func (c *Casino) cpuActionBeginner() int {
	// Try to make a capturing move.
	if cardIdx := c.tryCaptureMove(); cardIdx != -1 {
		return cardIdx
	}
	return -1 // No move found. Let the generic fallback in CPUaction handle it.
}

func (c *Casino) cpuActionIntermediate() int {
	// Try to make a capturing move.
	if cardIdx := c.tryCaptureMove(); cardIdx != -1 {
		return cardIdx
	}
	// Try to play a known "safe" card.
	if cardIdx := c.trySafeDiscard(); cardIdx != -1 {
		return cardIdx
	}
	// Find the most common non-Jack card face considering both the CPU's hand
	// and the cards played in this hand, then discard it.
	faceCounts := make(map[string]int)
	// Count faces in the CPU's own hand.
	for _, card := range c.cpuCards {
		if card != nil && card.GetFace() != "Jack" { // Exclude Jacks.
			faceCounts[card.GetFace()]++
		}
	}
	// Count faces from the current hand memory(Short-term memory).
	for i := 0; i < c.currentHandMemoryLength; i++ {
		card := c.currentHandMemory[i]
		if card != nil && card.GetFace() != "Jack" { // Exclude Jacks.
			faceCounts[card.GetFace()]++
		}
	}
	// Find the most common face among the cards the CPU holds.
	mostCommonFace := ""
	maxCount := 1 // Only care if a face appears more than once (i.e., is a "safer" discard).
	for _, card := range c.cpuCards {
		if card != nil {
			if count := faceCounts[card.GetFace()]; count > maxCount {
				maxCount = count
				mostCommonFace = card.GetFace()
			}
		}
	}
	// If a safe discard was found, play it.
	if mostCommonFace != "" {
		return c.findMatchingCard(mostCommonFace)
	}
	return -1 // // No move found. Let the generic fallback in CPUaction handle it.
}

func (c *Casino) cpuActionAdvanced() int {
	// 1. Try to make a capturing move.
	if cardIdx := c.tryCaptureMove(); cardIdx != -1 {
		return cardIdx
	}
	// Try to play a known "safe" card.
	if cardIdx := c.trySafeDiscard(); cardIdx != -1 {
		return cardIdx
	}
	// Play a card that maximizes its "match number" (frequency across all played cards + duplicates in hand).
	greatestMatchNumber := 0
	cardToPlay := -1
	for i := 0; i < HandSize; i++ {
		matchNumber := 0
		if c.cpuCards[i] != nil && c.cpuCards[i].GetFace() != "Jack" { // Exclude Jacks.
			// Count matches in all cards played memory(Long-term memory).
			for j := 0; j < c.allPlayedCardsMemoryLength; j++ {
				if c.allPlayedCardsMemory[j] != nil && c.allPlayedCardsMemory[j].GetFace() == c.cpuCards[i].GetFace() {
					matchNumber++
				}
			}
			// Count matches in the CPU's own hand.
			for j := 0; j < HandSize; j++ {
				if i == j || c.cpuCards[j] == nil {
					continue
				} else if c.cpuCards[j].GetFace() == c.cpuCards[i].GetFace() {
					matchNumber++
				}
			}
			if matchNumber > greatestMatchNumber {
				greatestMatchNumber = matchNumber
				cardToPlay = i
			}
		}
	}
	if greatestMatchNumber != 0 && cardToPlay != -1 {
		return cardToPlay
	}
	// If no strategic move is found, play the least valuable non-Jack card.
	leastValue := 100 // Start with a high value.
	cardToPlay = -1
	for i, card := range c.cpuCards {
		if card != nil && card.GetFace() != "Jack" { // Exclude Jacks.
			value := getCardValue(card)
			if value < leastValue {
				leastValue = value
				cardToPlay = i
			}
		}
	}
	if cardToPlay != -1 {
		return cardToPlay
	}
	return -1 // No move found. Let the generic fallback in CPUaction handle it.
}

// CPUaction determines which card the CPU should play based on the level.
func (c *Casino) CPUaction() int {
	var cardIdx int
	switch c.level {
	case LevelBeginner:
		cardIdx = c.cpuActionBeginner()
	case LevelIntermediate:
		cardIdx = c.cpuActionIntermediate()
	case LevelAdvanced:
		cardIdx = c.cpuActionAdvanced()
	default:
		cardIdx = -1 // Should not happen, but good practice.
	}
	// If the level-specific logic returns -1 (no strategic move found),
	// this is the final fallback to play any available card.
	// Must prioritize playing a non-Jack to avoid wasting a Jack on empty table.
	if cardIdx == -1 {
		// First, try to find any random non-Jack card.
		cardIdx = c.findRandomNonJack()
		if cardIdx != -1 {
			return cardIdx
		}
		// If no non-Jacks are found (i.e., the hand is only Jacks), find any card to play.
		for i, card := range c.cpuCards {
			if card != nil {
				return i // Return the first available card.
			}
		}
	}
	// Return the strategic move if one was found.
	return cardIdx
}

// pointCalculator calculates points from cards currently on the table.
func (c *Casino) pointCalculator() int {
	// This is an internal helper that calculates points from the current table pile.
	// It assumes the caller has already acquired the mutex lock.
	point := 0
	for i := 0; i < c.cardsOnTable; i++ {
		card := c.tableCards[i]
		switch card.GetFace() {
		case "Jack":
			point++
		case "Ace":
			point++
		case "Deuce":
			if card.GetSuit() == "Clubs" {
				point += 2
			}
		case "Ten":
			if card.GetSuit() == "Diamonds" {
				point += 3
			}
		}
	}
	return point
}

// getCardValue returns the point value of a single card.
func getCardValue(card *Card) int {
	if card == nil {
		return 0
	}
	switch card.GetFace() {
	case "Jack", "Ace":
		return 1
	case "Deuce":
		if card.GetSuit() == "Clubs" {
			return 2
		}
	case "Ten":
		if card.GetSuit() == "Diamonds" {
			return 3
		}
	}
	return 0
}

// undoImplementation reverts the last two plays (player and CPU).
func (c *Casino) undoImplementation() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.canUndo {
		// Cannot undo if a turn hasn't been played.
		return false
	}
	// Restore points and collection state.
	c.playerPoint = c.undoState.playerPoint
	c.cpuPoint = c.undoState.cpuPoint
	c.lastScorer = c.undoState.lastScorer
	c.cardsCollectedByPlayer = c.undoState.cardsCollectedByPlayer
	c.cardsCollectedByCPU = c.undoState.cardsCollectedByCPU
	// Restore the table pile.
	c.cardsOnTable = c.undoState.cardsOnTable
	// First, copy the saved cards back into the main slice.
	copy(c.tableCards, c.undoState.tableCards)
	// Then, nil out any subsequent slots to clear old data without shrinking the slice.
	for i := c.cardsOnTable; i < len(c.tableCards); i++ {
		if c.tableCards[i] == nil {
			break // Stop if we hit an already nil slot.
		}
		c.tableCards[i] = nil
	}
	// Restore player and CPU hands.
	copy(c.playerCards, c.undoState.playerCards)
	copy(c.cpuCards, c.undoState.cpuCards)
	c.initialHiddenCards = c.undoState.initialHiddenCards
	c.safeDiscardCandidate = c.undoState.safeDiscardCandidate
	c.isInitialPile = c.undoState.isInitialPile
	// Restore the Intermediate AI's memory.
	c.currentHandMemoryLength = c.undoState.currentHandMemoryLength
	copy(c.currentHandMemory, c.undoState.currentHandMemory)
	// Nil out any subsequent slots to clear old data.
	for i := c.currentHandMemoryLength; i < len(c.currentHandMemory); i++ {
		if c.currentHandMemory[i] == nil {
			break
		}
		c.currentHandMemory[i] = nil
	}
	// An undo can only be performed once per turn.
	c.canUndo = false
	return true
}
