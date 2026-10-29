package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test",
		Short: "Run automated integration test suite against connected dongle",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, cleanup, err := getService()
			if err != nil {
				return err
			}
			defer cleanup()

			fmt.Println("==================================================")
			fmt.Println("       Running Automated HSM Integration Suite     ")
			fmt.Println("==================================================")

			// 1. Echo Test
			fmt.Print("[1/5] Testing Echo Round-trip... ")
			msg := "TestPayload-123456"
			reply, lat, err := svc.Echo(msg)
			if err != nil || reply != msg {
				fmt.Printf("FAILED (%v)\n", err)
				return err
			}
			fmt.Printf("PASSED (%v)\n", lat)

			// 2. Device Info Test
			fmt.Print("[2/5] Testing Device Info Query... ")
			devInfo, err := svc.GetDeviceInfo()
			if err != nil {
				fmt.Printf("FAILED (%v)\n", err)
				return err
			}
			fmt.Printf("PASSED (v%s, Serial: 0x%08X)\n", devInfo.VersionString(), devInfo.Serial)

			// 3. Firmware Info Test
			fmt.Print("[3/5] Testing Firmware DFU Query... ")
			fwInfo, err := svc.GetFirmwareInfo()
			if err != nil {
				fmt.Printf("FAILED (%v)\n", err)
				return err
			}
			fmt.Printf("PASSED (Active %s, %d bytes)\n", fwInfo.SlotString(), fwInfo.ImageSize)

			// 4. Lock / Unlock State Transitions & LED
			fmt.Print("[4/5] Testing Unlock (Solid Green) & Lock (Solid Red)... ")
			if err := svc.Unlock(); err != nil {
				fmt.Printf("FAILED (%v)\n", err)
				return err
			}
			time.Sleep(200 * time.Millisecond)
			if err := svc.Lock(); err != nil {
				fmt.Printf("FAILED (%v)\n", err)
				return err
			}
			fmt.Println("PASSED")

			// 5. Reset to Unconfigured (Pulsating Blue)
			fmt.Print("[5/5] Testing Reset to Unconfigured (Pulsating Blue)... ")
			if err := svc.Reset(); err != nil {
				fmt.Printf("FAILED (%v)\n", err)
				return err
			}
			fmt.Println("PASSED")

			fmt.Println("==================================================")
			fmt.Println(" ✓ All integration tests completed successfully! ")
			fmt.Println("==================================================")
			return nil
		},
	}
}
