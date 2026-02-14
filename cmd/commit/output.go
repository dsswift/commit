package main

import "fmt"

// Console output helpers

func printStep(emoji, message string) {
	fmt.Printf("\n%s %s\n", emoji, message)
}

func printSuccess(message string) {
	fmt.Printf("   ✓ %s\n", message)
}

func printStepError(message string) {
	fmt.Printf("   ✗ %s\n", message)
}

func printProgress(message string) {
	fmt.Printf("   ⋯ %s\n", message)
}

func printVerbose(message string) {
	fmt.Printf("   │ %s\n", message)
}

func printWarning(message string) {
	fmt.Printf("   ⚠️  %s\n", message)
}

func printError(message string, err error) {
	fmt.Printf("   ✗ %s: %v\n", message, err)
}

func printFinal(emoji, message string) {
	fmt.Printf("\n%s %s\n", emoji, message)
}
