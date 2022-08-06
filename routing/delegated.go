package routing

import (
	"context"

	"github.com/ipfs/go-datastore"
	drc "github.com/ipfs/go-delegated-routing/client"
	drp "github.com/ipfs/go-delegated-routing/gen/proto"
	"github.com/ipfs/kubo/config"
	host "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/fullrt"
	record "github.com/libp2p/go-libp2p-record"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
)

type TieredRouter interface {
	routing.Routing
	ProvideMany() ProvideMany
}

var _ TieredRouter = &Tiered{}

// Tiered is a routing Tiered implementation providing some extra methods to fill
// some special use cases when initializing the client.
type Tiered struct {
	routinghelpers.Tiered
}

// ProvideMany returns a ProvideMany implementation including all Routers that
// implements ProvideMany
func (ds Tiered) ProvideMany() ProvideMany {
	var pms []ProvideMany
	for _, r := range ds.Tiered.Routers {
		pm, ok := r.(ProvideMany)
		if !ok {
			continue
		}
		pms = append(pms, pm)
	}

	if len(pms) == 0 {
		return nil
	}

	return &ProvideManyWrapper{pms: pms}
}

const defaultPriority = 100000

// GetPriority extract priority from config params.
// Small numbers represent more important routers.
func GetPriority(params config.RouterParams) int {
	param, ok := params.Number(config.RouterParamPriority)
	if !ok {
		return defaultPriority
	}

	return param
}

func ReframeRoutingFromConfig(conf config.Router) (routing.Routing, error) {
	var dr drp.DelegatedRouting_Client

	addr, ok := conf.Parameters.String(config.RouterParamEndpoint)
	if !ok {
		return nil, NewParamNeededErr(config.RouterParamEndpoint, conf.Type)
	}

	dr, err := drp.New_DelegatedRouting_Client(addr)
	if err != nil {
		return nil, err
	}

	c := drc.NewClient(dr)
	crc := drc.NewContentRoutingClient(c)
	return &reframeRoutingWrapper{
		Client:               c,
		ContentRoutingClient: crc,
	}, nil
}

type ExtraDHTParams struct {
	ExperimentalTrackFullNetworkDHT bool
	BootstrapPeers                  []peer.AddrInfo
	Host                            host.Host
	Validator                       record.Validator
	Datastore                       datastore.Batching
	Context                         context.Context
}

func DHTRoutingFromConfig(conf config.Router, params *ExtraDHTParams) (routing.Routing, error) {
	fullDHT, ok := conf.Parameters.Bool(config.RouterParamTrackFullNetworkDHT)
	if fullDHT && ok {
		return createFullRT(params)
	}

	dhtTP, _ := conf.Parameters.String(config.RouterParamDHTType)
	mode := dht.ModeAuto
	switch dhtTP {
	case config.RouterValueDHTType:
		mode = dht.ModeAuto
	case config.RouterValueDHTTypeClient:
		mode = dht.ModeClient
	case config.RouterValueDHTTypeServer:
		mode = dht.ModeServer
	case config.RouterValueDHTTypeNone:
		return routinghelpers.Null{}, nil
	default:
		return nil, &InvalidValueError{
			ParamName:    config.RouterParamDHTType,
			InvalidValue: dhtTP,
			ValidValues:  config.RouterValueDHTTypes,
		}
	}

	return createDHT(params, mode)
}

func createDHT(params *ExtraDHTParams, mode dht.ModeOpt) (routing.Routing, error) {
	return dht.New(
		params.Context, params.Host,
		dht.Concurrency(10),
		dht.Mode(mode),
		dht.Datastore(params.Datastore),
		dht.Validator(params.Validator),
		dht.BootstrapPeers(params.BootstrapPeers...),
	)
}

func createFullRT(params *ExtraDHTParams) (routing.Routing, error) {
	return fullrt.NewFullRT(params.Host,
		dht.DefaultPrefix,
		fullrt.DHTOption(
			dht.Validator(params.Validator),
			dht.Datastore(params.Datastore),
			dht.BootstrapPeers(params.BootstrapPeers...),
			dht.BucketSize(20),
		),
	)
}
