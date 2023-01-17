package libp2p

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
)

var _ network.ResourceManager = (*backpressureResourceManager)(nil)

type backpressureResourceManager struct {
	delegate    network.ResourceManager
	connCount   int64
	streamCount int64
}

// ViewSystem views the system wide resource scope.
// The system scope is the top level scope that accounts for global
// resource usage at all levels of the system. This scope constrains all
// other scopes and institutes global hard limits.
func (bprm *backpressureResourceManager) ViewSystem(f func(network.ResourceScope) error) error {
	return bprm.delegate.ViewSystem(f)
}

// ViewTransient views the transient (DMZ) resource scope.
// The transient scope accounts for resources that are in the process of
// full establishment.  For instance, a new connection prior to the
// handshake does not belong to any peer, but it still needs to be
// constrained as this opens an avenue for attacks in transient resource
// usage. Similarly, a stream that has not negotiated a protocol yet is
// constrained by the transient scope.
func (bprm *backpressureResourceManager) ViewTransient(f func(network.ResourceScope) error) error {
	return bprm.delegate.ViewTransient(f)
}

// ViewService retrieves a service-specific scope.
func (bprm *backpressureResourceManager) ViewService(svc string, f func(network.ServiceScope) error) error {
	return bprm.delegate.ViewService(svc, f)
}

// ViewProtocol views the resource management scope for a specific protocol.
func (bprm *backpressureResourceManager) ViewProtocol(p protocol.ID, f func(network.ProtocolScope) error) error {
	return bprm.delegate.ViewProtocol(p, f)
}

// ViewPeer views the resource management scope for a specific peer.
func (bprm *backpressureResourceManager) ViewPeer(peer peer.ID, f func(network.PeerScope) error) error {
	return bprm.delegate.ViewPeer(peer, f)
}

// OpenConnection creates a new connection scope not yet associated with any peer; the connection
// is scoped at the transient scope.
// The caller owns the returned scope and is responsible for calling Done in order to signify
// the end of the scope's span.
func (bprm *backpressureResourceManager) OpenConnection(dir network.Direction, usefd bool, endpoint multiaddr.Multiaddr) (network.ConnManagementScope, error) {
	atomic.AddInt64(&bprm.connCount, 1)

	for {
		cms, err := bprm.delegate.OpenConnection(dir, usefd, endpoint)
		if err == nil {
			atomic.AddInt64(&bprm.connCount, -1)
			return cms, nil
		}

		fmt.Println("OPENING CONNECTION ERROR, RETRYING", err, bprm.connCount)
		<-time.After(1 * time.Second)
		fmt.Println("RETRYING CONNECTION", bprm.connCount)
	}
}

// OpenStream creates a new stream scope, initially unnegotiated.
// An unnegotiated stream will be initially unattached to any protocol scope
// and constrained by the transient scope.
// The caller owns the returned scope and is responsible for calling Done in order to signify
// the end of th scope's span.
func (bprm *backpressureResourceManager) OpenStream(p peer.ID, dir network.Direction) (network.StreamManagementScope, error) {
	atomic.AddInt64(&bprm.streamCount, 1)

	for {
		str, err := bprm.delegate.OpenStream(p, dir)
		if err == nil {
			atomic.AddInt64(&bprm.streamCount, -1)
			return str, nil
		}

		fmt.Println("OPENING STREAM ERROR, RETRYING", err, bprm.streamCount)
		<-time.After(1 * time.Second)
		fmt.Println("RETRYING STREAM", bprm.streamCount)
	}
}

// Close closes the resource manager
func (bprm *backpressureResourceManager) Close() error {
	return bprm.delegate.Close()
}

var _ network.StreamManagementScope = (*streamManagerScope)(nil)

type streamManagerScope struct {
	delegate network.StreamManagementScope
	*resourceScopeSpan
}

// ProtocolScope returns the protocol resource scope associated with this stream.
// It returns nil if the stream is not associated with any protocol scope.
func (sms *streamManagerScope) ProtocolScope() network.ProtocolScope {
	return sms.delegate.ProtocolScope()
}

// SetProtocol sets the protocol for a previously unnegotiated stream
func (sms *streamManagerScope) SetProtocol(proto protocol.ID) error {
	return sms.delegate.SetProtocol(proto)
}

// ServiceScope returns the service owning the stream, if any.
func (sms *streamManagerScope) ServiceScope() network.ServiceScope {
	return sms.delegate.ServiceScope()
}

// SetService sets the service owning this stream.
func (sms *streamManagerScope) SetService(srv string) error {
	return sms.delegate.SetService(srv)
}

// PeerScope returns the peer resource scope associated with this stream.
func (sms *streamManagerScope) PeerScope() network.PeerScope {
	return sms.delegate.PeerScope()
}

var _ network.ConnManagementScope = (*connManagerScope)(nil)

type connManagerScope struct {
	delegate network.ConnManagementScope
	*resourceScopeSpan
}

// PeerScope returns the peer scope associated with this connection.
// It returns nil if the connection is not yet asociated with any peer.
func (cms *connManagerScope) PeerScope() network.PeerScope {
	return cms.delegate.PeerScope()
}

// SetPeer sets the peer for a previously unassociated connection
func (cms *connManagerScope) SetPeer(pid peer.ID) error {
	return cms.delegate.SetPeer(pid)
}

var _ network.ResourceScopeSpan = (*resourceScopeSpan)(nil)

type resourceScopeSpan struct {
	delegate network.ResourceScopeSpan
	counter  int64
}

// ReserveMemory reserves memory/buffer space in the scope; the unit is bytes.
// If ReserveMemory returns an error, then no memory was reserved and the caller should handle
// the failure condition.
//
// The priority argument indicates the priority of the memory reservation. A reservation
// will fail if the available memory is less than (1+prio)/256 of the scope limit, providing
// a mechanism to gracefully handle optional reservations that might overload the system.
// For instance, a muxer growing a window buffer will use a low priority and only grow the buffer
// if there is no memory pressure in the system.
//
// The are 4 predefined priority levels, Low, Medium, High and Always,
// capturing common patterns, but the user is free to use any granularity applicable to his case.
func (rss *resourceScopeSpan) ReserveMemory(size int, prio uint8) error {
	err := rss.delegate.ReserveMemory(size, prio)
	if err != nil {
		fmt.Println("++++++++++++++++++++++++++++++++++++++ RESERVE MEMORY ERROR", err)
	}
	return err
}

// ReleaseMemory explicitly releases memory previously reserved with ReserveMemory
func (rss *resourceScopeSpan) ReleaseMemory(size int) {
	rss.delegate.ReleaseMemory(size)
}

// Stat retrieves current resource usage for the scope.
func (rss *resourceScopeSpan) Stat() network.ScopeStat {
	return rss.delegate.Stat()
}

// BeginSpan creates a new span scope rooted at this scope
func (rss *resourceScopeSpan) BeginSpan() (network.ResourceScopeSpan, error) {
	for {
		span, err := rss.delegate.BeginSpan()
		if err == nil {
			atomic.AddInt64(&rss.counter, -1)
			return span, nil
		}

		atomic.AddInt64(&rss.counter, 1)

		fmt.Println("BEGIN SPAN, RETRYING", err, rss.counter)
		time.Sleep(1 * time.Second)
		fmt.Println("RETRYING SPAN", rss.counter)

	}
}

// Done ends the span and releases associated resources.
func (rss *resourceScopeSpan) Done() {
	rss.delegate.Done()
}
