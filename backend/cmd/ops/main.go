// Command ops groups the operator tasks: migrations, JWT key management and
// one-off job runs.
//
//	ops migrate up|down [n]|version
//	ops keys generate --dir DIR --kid KID
//	ops job run NAME
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"opensex/backend/internal/app"
	"opensex/backend/internal/config"
	"opensex/backend/internal/database"
	"opensex/backend/internal/devseed"
	"opensex/backend/internal/jobs"
	"opensex/backend/pkg/jwt"
	"opensex/backend/pkg/payment"
)

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  ops migrate up | down [n] | version     (DATABASE_URL, role app_migrator)
  ops keys generate --dir DIR --kid KID   (Ed25519 key pair for JWT signing)
  ops job run NAME                        (renewal_invoices, expire_periods, reconcile_payments, recompute_publication, purge)
  ops health check URL                    (container healthcheck, exits 1 if not 200)
  ops dev seed                            (development only: demo houses, profiles, clients, payments)
  ops dev ipn PAYMENT_ID STATUS [API_URL] (development only: signed IPN for the fake provider;
                                          STATUS = finished | partially_paid | expired ...)`)
	os.Exit(2)
}

func main() {
	if len(os.Args) < 3 {
		usage()
	}
	if err := run(os.Args[1], os.Args[2], os.Args[3:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(group, cmd string, args []string) error {
	switch group {
	case "migrate":
		url := os.Getenv("DATABASE_URL")
		if url == "" {
			return fmt.Errorf("DATABASE_URL is required")
		}
		switch cmd {
		case "up":
			return database.MigrateUp(url)
		case "down":
			n := 1
			if len(args) > 0 {
				if args[0] == "all" {
					n = 0
				} else if v, err := strconv.Atoi(args[0]); err == nil {
					n = v
				}
			}
			return database.MigrateDown(url, n)
		case "version":
			v, dirty, err := database.MigrateVersion(url)
			if err != nil {
				return err
			}
			fmt.Printf("version=%d dirty=%v\n", v, dirty)
			return nil
		}
	case "keys":
		if cmd == "generate" {
			fs := flag.NewFlagSet("keys generate", flag.ExitOnError)
			dir := fs.String("dir", os.Getenv("JWT_KEYS_DIR"), "keys directory")
			kid := fs.String("kid", "", "key id (e.g. 2026-11)")
			_ = fs.Parse(args)
			if *dir == "" || *kid == "" {
				return fmt.Errorf("--dir and --kid are required")
			}
			if err := jwt.GenerateKeyFiles(*dir, *kid); err != nil {
				return err
			}
			fmt.Printf("generated %s/%s.pem (private) and %s.pub.pem; deploy it as verification key first, then set JWT_ACTIVE_KID=%s\n", *dir, *kid, *kid, *kid)
			return nil
		}
	case "health":
		if cmd == "check" && len(args) == 1 {
			client := &http.Client{Timeout: 3 * time.Second}
			resp, err := client.Get(args[0])
			if err != nil {
				return err
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("status %d", resp.StatusCode)
			}
			return nil
		}
	case "dev":
		if cmd == "ipn" && len(args) >= 2 {
			return devIPN(args)
		}
		if cmd == "seed" {
			return devSeed()
		}
	case "job":
		if cmd == "run" && len(args) == 1 {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			ctx := context.Background()
			a, err := app.New(ctx, cfg)
			if err != nil {
				return err
			}
			defer a.Close()
			for _, j := range jobs.Jobs(a.Svc) {
				if j.Name == args[0] {
					n, skipped, err := jobs.RunOnce(ctx, a.Svc, j)
					fmt.Printf("job=%s changes=%d skipped=%v\n", j.Name, n, skipped)
					return err
				}
			}
			return fmt.Errorf("unknown job %q", args[0])
		}
	}
	usage()
	return nil
}

// devIPN posts a signed IPN (fake provider) to simulate a crypto payment.
func devIPN(args []string) error {
	if env := os.Getenv("APP_ENV"); env != "" && env != config.EnvDevelopment {
		return fmt.Errorf("dev ipn is only available in development")
	}
	secret := os.Getenv("NOWPAYMENTS_IPN_SECRET")
	if secret == "" {
		secret = "fake-ipn-secret"
	}
	api := "http://127.0.0.1:8080/api/v1/webhooks/nowpayments"
	if len(args) > 2 {
		api = args[2]
	}
	body, _ := json.Marshal(map[string]any{
		"payment_id": json.Number(strconv.FormatInt(time.Now().UnixNano()/1000, 10)), "payment_status": args[1], "order_id": args[0],
		"price_amount": json.Number("500"), "price_currency": "eur", "pay_amount": json.Number("0.0123"),
		"actually_paid": json.Number("0.0123"), "pay_currency": "btc",
	})
	sig, err := payment.SignIPN([]byte(secret), body)
	if err != nil {
		return err
	}
	req, _ := http.NewRequest(http.MethodPost, api, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-nowpayments-sig", sig)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	fmt.Printf("%d %s\n", resp.StatusCode, out)
	return nil
}

// devSeed loads the demo data set (internal/devseed).
func devSeed() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	a, err := app.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer a.Close()
	start := time.Now()
	r, err := devseed.Run(ctx, a.Svc)
	if err != nil {
		return err
	}
	fmt.Printf("demo data loaded in %s: %d houses, %d attested profiles, %d pending registrations, %d clients, %d favorites\n",
		time.Since(start).Round(time.Second), r.Establishments, r.Profiles, r.Pending, r.Clients, r.Favorites)
	fmt.Printf("every demo account uses the password %q (managers: rose_gerant, velours_gerant, lune_gerant, jade_gerant, opale_gerant, nacre_gerant; clients: client_alex …)\n", devseed.Password)
	return nil
}
