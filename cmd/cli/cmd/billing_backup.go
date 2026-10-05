package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"memdoor/gateway/billingsvc"
	"memdoor/pkg/shared"

	"github.com/spf13/cobra"
)

// memdoor billing backup — the nightly copy (gateway/billingsvc/backup.go).
//
// Installed as memdoor-backup.timer by scripts/deploy.sh. With --rsync (or
// MEMDOOR_BACKUP_RSYNC in billing.env) the folder of backups is mirrored
// off the machine after each run; without it the copies live on the same
// disk as the original, which survives a bad deploy and a bad migration
// but not the machine.
var billingBackupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Copy the ledger and broker state into a dated folder, verify it, prune old ones",
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath, _ := cmd.Flags().GetString("db")
		if dbPath == "" {
			dbPath = shared.MemdoorHome("billing.db")
		}
		out, _ := cmd.Flags().GetString("out")
		keep, _ := cmd.Flags().GetInt("keep")
		rep, err := billingsvc.Backup(dbPath, out, keep, time.Now())
		if err != nil {
			return err
		}
		fmt.Println(rep.String())
		target, _ := cmd.Flags().GetString("rsync")
		if target == "" {
			target = os.Getenv("MEMDOOR_BACKUP_RSYNC")
		}
		if target == "" {
			fmt.Println("not mirrored off this machine: set MEMDOOR_BACKUP_RSYNC (user@host:path) in billing.env")
			return nil
		}
		rs := exec.Command("rsync", "-a", "--delete", out+"/", target)
		if o, err := rs.CombinedOutput(); err != nil {
			return fmt.Errorf("mirror to %s: %v: %s", target, err, string(o))
		}
		fmt.Printf("mirrored to %s\n", target)
		return nil
	},
}

func init() {
	billingBackupCmd.Flags().String("db", "", "Billing database path (default ~/.memdoor/billing.db)")
	billingBackupCmd.Flags().String("out", "/var/backups/memdoor", "Where dated backup folders go")
	billingBackupCmd.Flags().Int("keep", 30, "How many dated folders to keep")
	billingBackupCmd.Flags().String("rsync", "", "Mirror the backup folder here after each run (user@host:path); default $MEMDOOR_BACKUP_RSYNC")
	billingCmd.AddCommand(billingBackupCmd)
}
