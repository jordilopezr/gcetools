// Package pricing estimates list-price monthly cost from the public Cloud
// Billing Catalog (Compute Engine service). Estimates are on-demand list
// prices: no sustained-use, committed-use or negotiated discounts.
package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gcetools/internal/i18n"

	billing "google.golang.org/api/cloudbilling/v1"
	"google.golang.org/api/option"
)

const computeService = "services/6F81-5844-456A"

// HoursPerMonth is Google's convention for monthly estimates.
const HoursPerMonth = 730.0

// SKU is the subset of a catalog SKU needed for matching.
type SKU struct {
	Desc      string   `json:"d"`
	Family    string   `json:"f"` // Compute, Storage, Network, License
	UsageType string   `json:"u"` // OnDemand, Preemptible, Commit1Yr…
	Unit      string   `json:"n"` // h, GiBy.h, GiBy.mo, mo
	Price     float64  `json:"p"` // last (steady-state) tier, in Currency
	Regions   []string `json:"r"`
}

type Catalog struct {
	Currency string    `json:"currency"`
	Fetched  time.Time `json:"fetched"`
	SKUs     []SKU     `json:"skus"`
}

// Load returns the catalog from the local cache if younger than maxAge,
// otherwise downloads it with the first client that works.
func Load(ctx context.Context, currency string, maxAge time.Duration, clients ...*http.Client) (*Catalog, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		currency = "USD"
	}
	path := cachePath(currency)
	if c, err := readCache(path); err == nil && time.Since(c.Fetched) < maxAge {
		return c, nil
	}
	var lastErr error
	for _, hc := range clients {
		if hc == nil {
			continue
		}
		c, err := fetch(ctx, hc, currency)
		if err != nil {
			lastErr = err
			continue
		}
		_ = writeCache(path, c)
		return c, nil
	}
	return nil, fmt.Errorf(i18n.T("no se pudo descargar el catálogo de precios: %w"), lastErr)
}

func fetch(ctx context.Context, hc *http.Client, currency string) (*Catalog, error) {
	svc, err := billing.NewService(ctx, option.WithHTTPClient(hc))
	if err != nil {
		return nil, err
	}
	c := &Catalog{Currency: currency, Fetched: time.Now()}
	err = svc.Services.Skus.List(computeService).CurrencyCode(currency).PageSize(5000).Context(ctx).
		Pages(ctx, func(page *billing.ListSkusResponse) error {
			for _, s := range page.Skus {
				if s.Category == nil || len(s.PricingInfo) == 0 || s.PricingInfo[0].PricingExpression == nil {
					continue
				}
				pe := s.PricingInfo[0].PricingExpression
				if len(pe.TieredRates) == 0 || pe.TieredRates[len(pe.TieredRates)-1].UnitPrice == nil {
					continue
				}
				up := pe.TieredRates[len(pe.TieredRates)-1].UnitPrice
				c.SKUs = append(c.SKUs, SKU{
					Desc: s.Description, Family: s.Category.ResourceFamily, UsageType: s.Category.UsageType,
					Unit: pe.UsageUnit, Price: float64(up.Units) + float64(up.Nanos)/1e9, Regions: s.ServiceRegions,
				})
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	return c, nil
}

func cachePath(currency string) string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "gcetools", "compute-skus-"+currency+".json")
}

func readCache(path string) (*Catalog, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Catalog
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	if len(c.SKUs) == 0 {
		return nil, fmt.Errorf("caché vacía")
	}
	return &c, nil
}

func writeCache(path string, c *Catalog) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
