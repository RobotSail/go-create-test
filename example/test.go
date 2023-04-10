package main

import (
	"example.com/organization/repo/cmd"
	"fmt"
)

func fortnite() string {
	return "fortnite"
}

func rainbowSixSiege() string {
	for i := 0; i < 10; i++ {
		fmt.Println(i)
	}
	return "rainbowSixSiege"
}

func Games() []string {
	game1 := fortnite()
	game2 := rainbowSixSiege()

	return []string{game1, game2, cmd.Rocket()}
}
