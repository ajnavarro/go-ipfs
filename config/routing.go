package config

// Routing defines configuration options for libp2p routing
type Routing struct {
	// Type sets default daemon routing mode.
	//
	// Can be one of "dht", "dhtclient", "dhtserver", "none", or unset.
	Type *OptionalString `json:",omitempty"`

	Routers map[string]Router
}

type Router struct {

	// Currenly only supported Types are "reframe" and "dht".
	// Reframe type allows to add other resolvers using the Reframe spec:
	// https://github.com/ipfs/specs/tree/main/reframe
	// In the future we will support "dht" and other Types here.
	Type string

	Enabled Flag `json:",omitempty"`

	// Parameters are extra configuration that this router might need.
	// A common one for reframe router is "Endpoint".
	Parameters RouterParams
}

type RouterParams map[string]interface{}

func (rp RouterParams) String(key RouterParam) (string, bool) {
	out, ok := rp[string(key)].(string)
	return out, ok
}

func (rp RouterParams) Number(key RouterParam) (int, bool) {
	out, ok := rp[string(key)].(int)
	return out, ok
}

func (rp RouterParams) StringSlice(key RouterParam) ([]string, bool) {
	out, ok := rp[string(key)].([]string)
	return out, ok
}

func (rp RouterParams) Bool(key RouterParam) (bool, bool) {
	out, ok := rp[string(key)].(bool)
	return out, ok
}

// Type is the routing type.
// Depending of the type we need to instantiate different Routing implementations.
const (
	RouterTypeReframe = "reframe"
	RouterTypeDHT     = "dht"
)

type RouterParam string

const (
	// RouterParamEndpoint is the URL where the routing implementation will point to get the information.
	// Usually used for reframe Routers.
	RouterParamEndpoint            RouterParam = "Endpoint"
	RouterParamPriority            RouterParam = "Priority"
	RouterParamDHTType             RouterParam = "Mode"
	RouterParamTrackFullNetworkDHT RouterParam = "TrackFullNetworkDHT"
	RouterParamBootstrappers       RouterParam = "Bootstrappers"
	RouterParamPublicIPNetwork     RouterParam = "Public-IP-Network"
)

const (
	RouterValueDHTTypeServer = "dhtserver"
	RouterValueDHTTypeClient = "dhtclient"
	RouterValueDHTTypeNone   = "none"
	RouterValueDHTType       = "dht"
)

var RouterValueDHTTypes = []string{string(RouterValueDHTType), string(RouterValueDHTTypeServer), string(RouterValueDHTTypeClient), string(RouterValueDHTTypeNone)}
