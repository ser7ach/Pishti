package main

import "fmt"

type Card struct {
	face     string
	suit     string
	iconPath string // Stores the icon identifier (e.g., "1") used to look up the card's .png image.
}

// NewCard is a constructor for the Card struct.
func NewCard(cardFace, cardSuit, iconPath string) *Card {
	return &Card{
		face:     cardFace,
		suit:     cardSuit,
		iconPath: iconPath,
	}
}

// GetIconPath returns the icon path of the card.
func (c *Card) GetIconPath() string {
	return c.iconPath
}

// GetFace returns the face of the card.
func (c *Card) GetFace() string {
	return c.face
}

// GetSuit returns the suit of the card.
func (c *Card) GetSuit() string {
	return c.suit
}

// String representation for debugging
func (c *Card) String() string {
	return fmt.Sprintf("%s of %s", c.face, c.suit)
}
