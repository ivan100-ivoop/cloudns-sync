package dns

import "context"

// ZoneSource discovers authoritative zones from a local DNS configuration.
type ZoneSource interface {
	ListZones(ctx context.Context) ([]string, error)
}
