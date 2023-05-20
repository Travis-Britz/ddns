package ddns

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/netip"
	"strings"

	"github.com/cloudflare/cloudflare-go"
)

func newCloudflareProvider(token string) (cf *cloudflareProvider, err error) {
	cf = new(cloudflareProvider)
	cf.api, err = cloudflare.NewWithAPIToken(token)
	if err != nil {
		return nil, fmt.Errorf("error creating cloudflare api client: %w", err)
	}
	cf.logger = discard
	cf.comment = "managed by ddns"
	return cf, err
}

// cloudflareProvider implements ddns.Provider.
//
// It should be constructed using NewCloudflareProvider.
type cloudflareProvider struct {
	api    *cloudflare.API
	logger *log.Logger
	// cache *cache
	comment string // optional comment to attach to each new DNS entry
}

func (cf *cloudflareProvider) SetDNSRecords(ctx context.Context, domain string, addrs []netip.Addr) error {

	// this nil check feels odd and redundant, but it's technically possible for someone to use the type directly and cause a program crash.
	// should I just unexport CloudflareProvider and make the constructor return an interface or unexported type?
	if cf.api == nil {
		return errors.New("ddns.CloudflareProvider.SetDNSRecords: ddns.CloudflareProvider should be constructed with ddns.NewCloudflareProvider")
	}

	zid, err := cf.getZoneIDFromDomain(ctx, domain)
	if err != nil {
		return fmt.Errorf("unable to get zone ID for %s: %w", domain, err)
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

func (cf *cloudflareProvider) getZoneIDFromDomain(ctx context.Context, domain string) (zid string, err error) {
	zones, err := cf.api.ListZones(ctx)
	if err != nil {
		return "", fmt.Errorf("error listing zones: %w", err)
	}

	max := 0
	for _, z := range zones {
		if strings.HasSuffix(domain, z.Name) && len(z.Name) > max {
			max, zid = len(z.Name), z.ID
		}
	}
	if max == 0 {
		return "", fmt.Errorf("unable to find a zone matching \"%s\"", domain)
	}
	return zid, nil
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
