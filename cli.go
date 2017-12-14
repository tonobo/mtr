package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var (
	COUNT            = 5
	MAX_HOPS         = 64
	RING_BUFFER_SIZE = 8
)

// rootCmd represents the root command
var RootCmd = &cobra.Command{
	Use: "mtr TARGET",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("No target provided")
		}
		fmt.Println("Start:", time.Now())
		m := NewMTR(args[0])
		for i := 0; i < COUNT; i++ {
			m.Run()
		}
		m.Render()
		return nil
	},
}

func init() {
	RootCmd.Flags().IntVarP(&COUNT, "count", "c", COUNT, "Amount of pings per target")
	RootCmd.Flags().IntVar(&MAX_HOPS, "max-hops", MAX_HOPS, "Maximal TTL count")
	RootCmd.Flags().IntVar(&RING_BUFFER_SIZE, "buffer-size", RING_BUFFER_SIZE, "Cached packet buffer size")
}
