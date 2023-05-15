package ddns

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/netip"
	"strings"

	"github.com/cloudflare/cloudflare-go"
)

func NewCloudflareProvider(token string, logger *log.Logger) (cf *CloudflareProvider, err error) {
	cf.api, err = cloudflare.NewWithAPIToken(token)
	if err != nil {
		return nil, fmt.Errorf("error creating cloudflare api client: %w", err)
	}
	if cf.logger = logger; cf.logger == nil {
		cf.logger = log.New(io.Discard, "", log.LstdFlags)
	}
	cf.comment = "managed by ddns"
	return cf, err
}

type CloudflareProvider struct {
	api    *cloudflare.API
	logger *log.Logger
	// cache *cache
	comment string // optional comment to attach to each new DNS entry
}

func (cf *CloudflareProvider) SetDNSRecords(ctx context.Context, domain string, addrs []netip.Addr) error {

	sl := strings.Split(domain, ".")
	zone := strings.Join(sl[len(sl)-2:], ".")
	cf.logger.Printf("looking up zone ID for %s...\n", zone)
	zid, err := cf.api.ZoneIDByName(zone)
	if err != nil {
		return fmt.Errorf("unable to get zone ID for %s: %w", zone, err)
	}
	cf.logger.Printf("got zone ID: %s\n", zid)
	cf.logger.Printf("looking up A,AAAA records for zone %s...\n", zid)

	records, _, err := cf.api.ListDNSRecords(ctx, cloudflare.ZoneIdentifier(zid), cloudflare.ListDNSRecordsParams{
		Type:    "A,AAAA",
		Name:    domain,
		Content: "",
		Comment: "",
	})
	cf.logger.Printf("found %d existing records: %+v\n", len(records), records)
	existing := map[netip.Addr]bool{}
	newAddrs := map[netip.Addr]bool{}

	for _, a := range addrs {
		newAddrs[a] = true
	}
	for _, r := range records {
		a, err := netip.ParseAddr(r.Content)
		if err != nil {
			return fmt.Errorf("error parsing IP from content: %w", err)
		}
		existing[a] = true

		if _, found := newAddrs[a]; found {
			cf.logger.Printf("existing record %s is in the set of new addrs\n", a)
			continue
		}

		cf.logger.Printf("deleting DNS record for %s...\n", a)
		err = cf.api.DeleteDNSRecord(ctx, cloudflare.ZoneIdentifier(zid), r.ID)
		if err != nil {
			return fmt.Errorf("unable to delete DNS record %s: %w", r.ID, err)
		}
		cf.logger.Printf("successfully deleted record for %s\n", a)
	}

	for _, a := range addrs {
		if _, found := existing[a]; found {
			cf.logger.Printf("record already exists for %s\n", a)
			continue
		}
		cf.logger.Printf("creating record for %s...", a)
		record, err := cf.api.CreateDNSRecord(ctx, cloudflare.ZoneIdentifier(zid), cloudflare.CreateDNSRecordParams{
			Type:    recordType(a),
			Name:    domain,
			Content: a.String(),
			ZoneID:  zid,
			TTL:     60,
			Comment: cf.comment,
		})
		if err != nil {
			return fmt.Errorf("error creating DNS record: %w", err)
		}
		cf.logger.Printf("successfully added record: %+v\n", record)
	}

	return nil
}

func recordType(a netip.Addr) string {
	if a.Is4() {
		return "A"
	}
	if a.Is6() {
		return "AAAA"
	}
	panic("unknown ip configuration")
}
