package main

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// --- GUI Specific Code ---
// AppUI holds all the GUI widgets and the game state.
type AppUI struct {
	casino              *Casino
	isAnimating         bool // Flag to prevent clicks during CPU "turn" animation.
	gameOverSoundPlayed bool // Flag to ensure win/loss sound plays only once.
	// UI Components.
	window fyne.Window
	// Top bar.
	levelSelect *widget.Select
	startButton *widget.Button
	undoButton  *widget.Button
	// Center display.
	tableCardWidget *clickableImage
	tablePileImage  *canvas.Image
	infoLabel       *widget.Label
	// Player hands.
	playerCardWidgets []*clickableImage
	cpuCardWidgets    []*clickableImage
	// Score labels.
	playerScoreLabel *widget.Label
	cpuScoreLabel    *widget.Label
}

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("Pishti")
	// Set icon from file
	icon, err := fyne.LoadResourceFromPath("assets/ui/icon.png")
	if err == nil {
		myWindow.SetIcon(icon)
	}
	// Apply custom theme to make buttons and selects transparent.
	myApp.Settings().SetTheme(newTransparentTheme(myApp.Settings().Theme()))
	myWindow.SetFixedSize(true)
	myWindow.Resize(fyne.NewSize(440, 600))
	// Initialize all resources after the app is created to avoid deadlocks with Go tooling.
	loadResources()
	initAudio()
	ui := &AppUI{
		casino: NewCasino(),
		window: myWindow,
	}
	content := ui.buildLayout()
	ui.updateUI() // Initial UI state.
	myWindow.SetContent(content)
	myWindow.CenterOnScreen()
	// Add a confirmation dialog when the user tries to close the window.
	myWindow.SetCloseIntercept(func() {
		dialog.ShowConfirm("Exit", "Are you sure you want to quit?", func(confirmed bool) {
			if confirmed {
				myApp.Quit() // Quit the entire application.
			}
		}, myWindow)
	})
	myWindow.ShowAndRun()
}

func (ui *AppUI) buildLayout() fyne.CanvasObject {
	levelOptions := []string{"Beginner", "Intermediate", "Advanced"}
	// Top Bar.
	ui.levelSelect = widget.NewSelect(levelOptions, func(s string) {
		for i, label := range levelOptions {
			if s == label {
				// The GameLevel enum starts at 1 for Beginner, so add 1 to the index.
				ui.casino.SetLevel(GameLevel(i + 1))
				break
			}
		}
	})
	ui.levelSelect.PlaceHolder = "Select Level"
	// Fix for truncated text: Use a "probe" widget to get the correct size.
	// Create a temporary Select widget with the longest text to measure its minimum required size.
	// "Select Level" is the longest string in this case.
	probe := widget.NewSelect([]string{ui.levelSelect.PlaceHolder}, nil)
	probe.PlaceHolder = ui.levelSelect.PlaceHolder
	minWidgetWidth := probe.MinSize().Width
	// To set a minimum size, wrap the widget in a sized container.
	minWidgetSize := fyne.NewSize(minWidgetWidth, ui.levelSelect.MinSize().Height)
	// Use a container with a minSizeLayout to ensure the select widget meets a minimum width.
	sizedSelect := container.New(&minSizeLayout{min: minWidgetSize}, ui.levelSelect)
	ui.startButton = widget.NewButton("Start", func() {
		// If no game has started yet, just try to start one directly.
		// This handles both the very first game and subsequent games after a reset.
		if ui.casino.gameState == StateNotStarted {
			ui.attemptToStartGame()
			return
		}
		// If a game is over, the confirmation text should reflect that.
		if ui.casino.gameState == StateGameOver {
			dialog.ShowConfirm("New Game", "Are you sure you want to start a new game?", func(confirmed bool) {
				if confirmed {
					ui.resetGameUI()
				}
			}, ui.window)
		} else { // For any other state (an in-progress game).
			dialog.ShowConfirm("New Game", "Are you sure you want to end the current game?", func(confirmed bool) {
				if confirmed {
					ui.resetGameUI()
				}
			}, ui.window)
		}
	})
	ui.undoButton = widget.NewButton("Undo", func() {
		if ui.casino.undoImplementation() {
			// If a special message (like the initial pile capture) was being shown,
			// clear it now that the player has undone the action.
			if ui.casino.initialPileCaptureMsg != "" {
				ui.casino.initialPileCaptureMsg = ""
				ui.infoLabel.SetText("")
			}
			PlaySound(SoundUndo)
			ui.updateUI()
		}
	})
	// Score Labels are part of the top bar.
	ui.playerScoreLabel = widget.NewLabel("Your Score: 0")
	ui.playerScoreLabel.Alignment = fyne.TextAlignTrailing // Right-align for visual stability.
	ui.cpuScoreLabel = widget.NewLabel("CPU Score: 0")
	ui.cpuScoreLabel.Alignment = fyne.TextAlignTrailing // Right-align for visual stability.
	scoreBox := container.New(layout.NewVBoxLayout(), ui.playerScoreLabel, ui.cpuScoreLabel)
	// A Border layout is used here to get a thinner bar than HBox.
	// Group the left-side buttons together.
	leftButtons := container.New(layout.NewHBoxLayout(), sizedSelect, ui.startButton, ui.undoButton)
	topBarContent := container.New(layout.NewBorderLayout(nil, nil, leftButtons, scoreBox), leftButtons, scoreBox)
	// Create a semi-transparent background for the top bar.
	topBarBackground := canvas.NewRectangle(color.NRGBA{R: 0, G: 0, B: 0, A: 40}) // Barely visible black filter (~15% opacity).
	topBar := container.NewStack(topBarBackground, topBarContent)
	// The pile image sits behind the top card to represent face-down cards.
	ui.tablePileImage = canvas.NewImageFromResource(nil) // This is the second card in the pile.
	ui.tablePileImage.FillMode = canvas.ImageFillStretch // Stretch to fill the defined size.
	// The card image sits on top.
	// To give the table card a fixed size, wrap it in the custom clickableImage widget.
	// Give it a nil tap handler so it's not interactive.
	ui.tableCardWidget = newClickableImage(nil)
	ui.tableCardWidget.FillMode = canvas.ImageFillStretch // Stretch to fill the defined size.
	ui.infoLabel = widget.NewLabel("Welcome to Pishti! Select a level and start the game.")
	ui.infoLabel.Alignment = fyne.TextAlignCenter
	// To create the "peeking" card effect, use a container without a layout
	// and manually position the card images. Place the images directly in the container,
	// not inside other layout containers.
	tableStack := container.NewWithoutLayout(ui.tablePileImage, ui.tableCardWidget)
	tableStack.Resize(fyne.NewSize(76, 101)) // Card size (71x96) + 5px offset.
	// Position and size the top card.
	ui.tableCardWidget.Resize(fyne.NewSize(71, 96))
	ui.tableCardWidget.Move(fyne.NewPos(0, 0))
	// Position and size the peeking card underneath.
	ui.tablePileImage.Resize(fyne.NewSize(71, 96))
	ui.tablePileImage.Move(fyne.NewPos(5, 5)) // Offset by 5px down and right.
	// CPU Hand Area.
	ui.cpuCardWidgets = make([]*clickableImage, 4)
	cpuHandObjects := []fyne.CanvasObject{} // Use a slice to dynamically add cards and spacers.
	for i := 0; i < HandSize; i++ {
		// Use the custom widget, which now correctly reports its minimum size.
		// It's not clickable, so the onTapped handler is nil.
		ui.cpuCardWidgets[i] = newClickableImage(nil)
		ui.cpuCardWidgets[i].FillMode = canvas.ImageFillContain
		cardContainer := container.New(layout.NewCenterLayout(), ui.cpuCardWidgets[i])
		frameImage := canvas.NewImageFromResource(resourceFrame)
		frameImage.SetMinSize(fyne.NewSize(91, 116))
		cardSlot := container.NewStack(frameImage, cardContainer)
		cpuHandObjects = append(cpuHandObjects, cardSlot)
		// Add a spacer after each card, except the last one.
		if i < HandSize-1 {
			cpuHandObjects = append(cpuHandObjects, container.New(&minSizeLayout{min: fyne.NewSize(5, 0)}))
		}
	}
	cpuHandContainer := container.New(layout.NewHBoxLayout(), cpuHandObjects...)
	// The centerStack holds the vertically aligned game elements, without a background.
	// Add struts to create vertical space around the elements.
	topSpacer := container.New(&minSizeLayout{min: fyne.NewSize(0, 20)}, layout.NewSpacer())
	cpuArea := container.NewVBox(topSpacer, container.New(layout.NewCenterLayout(), cpuHandContainer))
	// Use a BorderLayout to perfectly center the table pile between the CPU hand and the info label.
	// A small spacer is added above the pile to push it down slightly for better visual balance.
	// Create a 40px high spacer using a container with a custom minSizeLayout.
	pileSpacer := container.New(&minSizeLayout{min: fyne.NewSize(0, 40)}, layout.NewSpacer())
	// Wrap the tableStack in a container with a fixed minSize to prevent the outer
	// layout from overriding the manual card positions.
	sizedTableStack := container.New(&minSizeLayout{min: tableStack.Size()}, tableStack)
	centerPileGroup := container.NewVBox(pileSpacer, container.New(layout.NewCenterLayout(), sizedTableStack)) // The VBox places the spacer above the pile
	// Use the NewBorder convenience function for a cleaner layout definition.
	centerStack := container.NewBorder(
		cpuArea, ui.infoLabel, nil, nil, // Top, Bottom, Left, Right.
		container.New(layout.NewCenterLayout(), centerPileGroup)) // Center.
	// Bottom Area (Player Hand).
	ui.playerCardWidgets = make([]*clickableImage, HandSize)
	playerHandObjects := []fyne.CanvasObject{} // Use a slice to dynamically add cards and spacers.
	for i := 0; i < HandSize; i++ {
		cardIndex := i
		frameImage := canvas.NewImageFromResource(resourceFrame)
		frameImage.SetMinSize(fyne.NewSize(91, 116))
		ui.playerCardWidgets[i] = newClickableImage(func() {
			// Only allow a play if:
			// 1. The card slot is not empty.
			// 2. No animation is in progress.
			// 3. It is currently the player's turn.
			if ui.casino.playerCards[cardIndex] != nil && !ui.isAnimating && ui.casino.gameState == StatePlayerTurn {
				ui.playerPlays(cardIndex)
			}
		})
		ui.playerCardWidgets[i].FillMode = canvas.ImageFillContain
		// Use a CenterLayout to position the card widget in the middle of the frame.
		cardSlot := container.NewStack(frameImage, container.NewCenter(ui.playerCardWidgets[i]))
		playerHandObjects = append(playerHandObjects, cardSlot)
		// Add a spacer after each card, except the last one.
		if i < HandSize-1 {
			playerHandObjects = append(playerHandObjects, container.New(&minSizeLayout{min: fyne.NewSize(5, 0)}))
		}
	}
	// The playerHand is a simple grid of card containers, without its own background.
	playerHand := container.New(layout.NewHBoxLayout(), playerHandObjects...)
	// Create a single background image for the entire window.
	backgroundImage := canvas.NewImageFromResource(resourceBackground)
	// Wrap the player hand in a CenterLayout to prevent it from being stretched by the BorderLayout.
	// Also add a strut below it for vertical spacing.
	bottomSpacer := container.New(&minSizeLayout{min: fyne.NewSize(0, 20)}, layout.NewSpacer())
	// Group the info label with the player's hand and the bottom spacer.
	bottomArea := container.NewVBox(playerHand, bottomSpacer)
	centeredPlayerHand := container.New(layout.NewCenterLayout(), bottomArea)
	// The mainLayout organizes all interactive elements.
	mainLayout := container.New(layout.NewBorderLayout(topBar, centeredPlayerHand, nil, nil),
		topBar, centeredPlayerHand, centerStack)
	return container.NewStack(backgroundImage, mainLayout)
}

// playerPlays orchestrates the sequence of events for a player's turn.
func (ui *AppUI) playerPlays(cardIndex int) {
	// If a special message (like the initial pile capture) is being shown,
	// clear it now that the player is taking a new action.
	if ui.casino.initialPileCaptureMsg != "" {
		ui.casino.initialPileCaptureMsg = ""
		ui.infoLabel.SetText("")
	}
	// 1. Lock the UI to prevent further clicks.
	ui.isAnimating = true
	// 2. Player makes their move in the game logic.
	ui.casino.playerPlays(cardIndex)
	fyne.Do(ui.updateUI) // Update UI to show player's card on the table.
	// 3. Wait briefly before the CPU makes its move.
	time.AfterFunc(1000*time.Millisecond, func() {
		ui.handleCPUTurn()
	})
}

// handleCPUTurn orchestrates the CPU's move and the subsequent state check.
func (ui *AppUI) handleCPUTurn() {
	// 1. CPU makes its move.
	ui.casino.cpuPlays()
	fyne.Do(ui.updateUI) // Update UI to show CPU's card.
	// 2. Decide what to do next based on the game state.
	if ui.casino.isHandFinished() {
		// The hand is over. Pause briefly, then process the end of the hand.
		// The UI remains locked until this is complete.
		time.AfterFunc(500*time.Millisecond, func() {
			ui.casino.checkEndOfHand()
			fyne.Do(ui.updateUI)
			ui.isAnimating = false // Unlock UI after new hand is dealt or game ends.
		})
	} else if ui.casino.gameState == StatePileCaptured {
		// A pile was captured. The updateUI function's timer will handle
		// finalizing the capture and unlocking the UI.
	} else {
		// It's a normal turn. Unlock the UI for the player's next move.
		ui.isAnimating = false
	}
}

// resetGameUI resets the game state and UI to the initial "welcome" screen.
func (ui *AppUI) resetGameUI() {
	ui.casino.ResetGame()
	ui.levelSelect.Enable()
	ui.levelSelect.ClearSelected()
	ui.startButton.SetText("Start")
	ui.gameOverSoundPlayed = false // Reset the flag for the next game.
	ui.updateUI()
}

// attemptToStartGame tries to start a new game, showing a warning if no level is selected.
func (ui *AppUI) attemptToStartGame() {
	if ui.casino.level == LevelNotSelected {
		ui.infoLabel.SetText("Please select a level first!")
		return
	}
	PlaySound(SoundGameStart)
	ui.casino.StartGame()
	ui.levelSelect.Disable()
	ui.startButton.SetText("New Game")
	ui.infoLabel.SetText("") // Clear the "Select a level..." message.
	ui.updateUI()
}

// updateHandUI is a helper to refresh the card widgets for a given hand.
func (ui *AppUI) updateHandUI(hand []*Card, widgets []*clickableImage, showFaceUp bool) {
	for i := 0; i < HandSize; i++ {
		card := hand[i]
		widget := widgets[i]
		if card != nil {
			if showFaceUp {
				widget.Resource = getCardResource(card)
			} else {
				widget.Resource = resourceCardBack
			}
		} else {
			widget.Resource = nil // Make card layer transparent.
		}
		// Always refresh to ensure the renderer processes the change.
		widget.Refresh()
		// Force an additional canvas refresh for the widget.
		canvas.Refresh(widget)
	}
}

func (ui *AppUI) updateUI() {
	c := ui.casino
	// Update scores.
	ui.playerScoreLabel.SetText(fmt.Sprintf("Your Score: %d", c.playerPoint))
	ui.cpuScoreLabel.SetText(fmt.Sprintf("CPU Score: %d", c.cpuPoint))
	// Update hands.
	ui.updateHandUI(c.cpuCards, ui.cpuCardWidgets, false)      // CPU hand is face-down.
	ui.updateHandUI(c.playerCards, ui.playerCardWidgets, true) // Player hand is face-up.
	// Update table image.
	if c.cardsOnTable > 0 {
		topCard := c.tableCards[c.cardsOnTable-1]
		res := getCardResource(topCard)
		ui.tableCardWidget.Resource = res
		// If there is more than one card, show the card underneath the top one
		// to indicate a pile.
		if c.cardsOnTable > 1 {
			// At the very start of the game, the cards under the top card are face down.
			if ui.casino.isInitialPile {
				ui.tablePileImage.Resource = resourceCardBack
			} else {
				secondCard := c.tableCards[c.cardsOnTable-2]
				ui.tablePileImage.Resource = getCardResource(secondCard)
			}
		} else {
			ui.tablePileImage.Resource = nil
			ui.tablePileImage.Image = nil
		}
	} else {
		ui.tableCardWidget.Resource = nil
		ui.tablePileImage.Resource = nil
		ui.tablePileImage.Image = nil // Clear the underlying image data.
	}
	ui.tableCardWidget.Refresh()
	ui.tablePileImage.Refresh()
	canvas.Refresh(ui.tablePileImage)
	// Update info label and button states.
	ui.undoButton.Disable() // Disabled by default.
	switch c.gameState {
	case StateNotStarted:
		ui.infoLabel.SetText("Select a level and press Start.")
	case StateGameOver:
		ui.startButton.Enable()
		// When the game is over, the user must click "New Game" to reset.
		if !ui.gameOverSoundPlayed {
			ui.handleGameOver()
		}
		ui.levelSelect.Disable()
	case StatePlayerTurn, StateCPUTurn:
		// Don't clear the info label here automatically. This allows messages like the
		// initial pile capture to persist until the player's next move clears it.
		if c.level != LevelAdvanced && c.canUndo {
			ui.undoButton.Enable()
		}
	case StatePileCaptured:
		// This state is a brief pause to show the captured pile before it's cleared.
		// If the initial pile was captured, show the special message.
		if c.initialPileCaptureMsg != "" {
			ui.infoLabel.SetText(c.initialPileCaptureMsg)
		}

		time.AfterFunc(500*time.Millisecond, func() {
			ui.casino.finalizeCapture()
			// The finalizeCapture function now sets the correct next game state.
			ui.isAnimating = false // Unlock the UI after the capture is complete.
			fyne.Do(ui.updateUI)
		})
	}
}

// handleGameOver sets the final game message, plays the win/loss sound, and sets a flag to prevent repeats.
func (ui *AppUI) handleGameOver() {
	c := ui.casino
	var gameOverMsg string
	var soundToPlay SoundEffect
	if c.playerPoint > c.cpuPoint {
		gameOverMsg = fmt.Sprintf("You Win! Final Score: You %d - %d CPU", c.playerPoint, c.cpuPoint)
		soundToPlay = SoundPlayerWins
	} else if c.cpuPoint > c.playerPoint {
		gameOverMsg = fmt.Sprintf("CPU Wins! Final Score: You %d - %d CPU", c.playerPoint, c.cpuPoint)
		soundToPlay = SoundCPUWins
	} else { // Tie
		gameOverMsg = fmt.Sprintf("It's a Tie! Final Score: You %d - %d CPU", c.playerPoint, c.cpuPoint)
		soundToPlay = SoundTie
	}
	PlaySound(soundToPlay)
	ui.infoLabel.SetText(gameOverMsg)
	ui.gameOverSoundPlayed = true // Set the flag to ensure this only runs once per game.
}
