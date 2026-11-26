package impure

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"
)

var counter int

var mu sync.Mutex

//tribrach:pure
func Clock() int64 { return time.Now().Unix() } // want "TRG0301 Clock uses time.Now; the model has no clock"

//tribrach:pure
func Dice() int { return rand.Intn(6) } // want "TRG0302 Dice uses math/rand.Intn; the model has no randomness"

//tribrach:pure
func Read() string {
	b, _ := os.ReadFile("x") // want "TRG0303 Read uses os.ReadFile; the model does no I/O"
	fmt.Println("read")      // want "TRG0303 Read uses fmt.Println"
	return string(b)
}

//tribrach:pure
func Bump() int {
	counter++ // want "TRG0304 Bump uses the package variable example.com/impure.counter"
	return counter
}

//tribrach:pure
func Spawn(ch chan int) {
	go func() {}() // want "TRG0305 Spawn uses a goroutine"
	ch <- 1        // want "TRG0305 Spawn uses a channel send"
	<-ch           // want "TRG0305 Spawn uses a channel receive"
	mu.Lock()      // want "TRG0304 Spawn uses the package variable example.com/impure.mu" "TRG0305 Spawn uses \\(\\*sync.Mutex\\).Lock"
}

//tribrach:pure
func Calls() int { return helper() } // want "TRG0307 Calls uses example.com/impure.helper; everything a linked function calls must be pure too"

func helper() int { return 1 }

// Formatting is pure, so Sprintf is fine.
//
//tribrach:pure
func Label(n int) string { return fmt.Sprintf("%d", n) }
