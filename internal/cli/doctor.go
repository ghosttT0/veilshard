package cli

import (
	"context"
	"flag"
	"fmt"

	"github.com/veilshard/veilshard/internal/doctor"
)

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	explainCode := fs.String("explain", "", "Provide detailed guidance for a specific rule ID (e.g., SSH_RULE, SYS_CLOCK)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	ui := NewUI()
	d := doctor.New()

	// If explain is requested
	if *explainCode != "" {
		item, err := d.Explain(*explainCode)
		if err != nil {
			ui.Error(err.Error())
			return err
		}
		ui.Header(fmt.Sprintf("Rule Explanation: %s", item.ID))
		ui.KeyValue("Category", item.Category)
		ui.KeyValue("Name", item.Name)
		fmt.Printf("\n%sGuidance & Recommendation:%s\n%s\n\n", ColorBold, ColorReset, item.Recommendation)
		return nil
	}

	ui.Header("vpnctl doctor")

	ctx := context.Background()
	rep, err := d.Run(ctx)
	if err != nil {
		ui.Error(fmt.Sprintf("doctor run failed: %v", err))
		return err
	}

	// Group by category
	categories := []string{"System", "Network", "Security", "Service"}
	for _, cat := range categories {
		ui.Section(cat)
		for _, it := range rep.Items {
			if it.Category == cat {
				switch it.Status {
				case doctor.StatusPass:
					ui.Step(it.Name, true, it.Message)
				case doctor.StatusWarn:
					ui.Warn(fmt.Sprintf("%-28s %s%s%s", it.Name, ColorGray, it.Message, ColorReset))
				case doctor.StatusFail:
					ui.Step(it.Name, false, it.Message)
				}
			}
		}
	}

	ui.Section("Doctor Result")
	fmt.Printf("%sPASS:%s %d\n", ColorGreen, ColorReset, rep.Pass)
	fmt.Printf("%sWARN:%s %d\n", ColorYellow, ColorReset, rep.Warn)
	fmt.Printf("%sFAIL:%s %d\n\n", ColorRed, ColorReset, rep.Fail)

	// Print recommendations for any WARN or FAIL items
	hasTips := false
	for _, it := range rep.Items {
		if (it.Status == doctor.StatusWarn || it.Status == doctor.StatusFail) && it.Recommendation != "" {
			if !hasTips {
				fmt.Printf("%sRecommendations:%s\n", ColorBold, ColorReset)
				hasTips = true
			}
			fmt.Printf("• [%s] %s\n  %s\n", it.ID, it.Name, it.Recommendation)
		}
	}
	if hasTips {
		fmt.Printf("\nTo inspect a rule in detail, run: %svpnctl doctor --explain <RULE_ID>%s\n\n", ColorCyan, ColorReset)
	}

	return nil
}
