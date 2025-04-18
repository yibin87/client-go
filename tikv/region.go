// Copyright 2021 TiKV Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// NOTE: The code in this file is based on code from the
// TiDB project, licensed under the Apache License v 2.0
//
// https://github.com/pingcap/tidb/tree/cc5e161ac06827589c4966674597c137cc9e809c/store/tikv/region.go
//

// Copyright 2021 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tikv

import (
	"time"

	"github.com/pingcap/kvproto/pkg/metapb"
	"github.com/tikv/client-go/v2/internal/apicodec"
	"github.com/tikv/client-go/v2/internal/client"
	"github.com/tikv/client-go/v2/internal/locate"
	"github.com/tikv/client-go/v2/kv"
	"github.com/tikv/client-go/v2/oracle"
	"github.com/tikv/client-go/v2/tikvrpc"
	pd "github.com/tikv/pd/client"
	"github.com/tikv/pd/client/pkg/circuitbreaker"
)

// RPCContext contains data that is needed to send RPC to a region.
type RPCContext = locate.RPCContext

// RPCCanceller is rpc send cancelFunc collector.
type RPCCanceller = locate.RPCCanceller

// RegionVerID is a unique ID that can identify a Region at a specific version.
type RegionVerID = locate.RegionVerID

// RegionCache caches Regions loaded from PD.
type RegionCache = locate.RegionCache

// KeyLocation is the region and range that a key is located.
type KeyLocation = locate.KeyLocation

// RPCCancellerCtxKey is context key attach rpc send cancelFunc collector to ctx.
type RPCCancellerCtxKey = locate.RPCCancellerCtxKey

// RegionRequestSender sends KV/Cop requests to tikv server. It handles network
// errors and some region errors internally.
//
// Typically, a KV/Cop request is bind to a region, all keys that are involved
// in the request should be located in the region.
// The sending process begins with looking for the address of leader store's
// address of the target region from cache, and the request is then sent to the
// destination tikv server over TCP connection.
// If region is updated, can be caused by leader transfer, region split, region
// merge, or region balance, tikv server may not able to process request and
// send back a RegionError.
// RegionRequestSender takes care of errors that does not relevant to region
// range, such as 'I/O timeout', 'NotLeader', and 'ServerIsBusy'. For other
// errors, since region range have changed, the request may need to split, so we
// simply return the error to caller.
type RegionRequestSender = locate.RegionRequestSender

// StoreSelectorOption configures storeSelectorOp.
type StoreSelectorOption = locate.StoreSelectorOption

// RegionRequestRuntimeStats records the runtime stats of send region requests.
type RegionRequestRuntimeStats = locate.RegionRequestRuntimeStats

// RPCRuntimeStats indicates the RPC request count and consume time.
type RPCRuntimeStats = locate.RPCRuntimeStats

// CodecPDClient wraps a PD Client to decode the encoded keys in region meta.
type CodecPDClient = locate.CodecPDClient

// NewCodecPDClient is a constructor for CodecPDClient
var NewCodecPDClient = locate.NewCodecPDClient

// NewCodecPDClientWithKeyspace creates a CodecPDClient in API v2 with keyspace name.
var NewCodecPDClientWithKeyspace = locate.NewCodecPDClientWithKeyspace

// NewCodecV1 is a constructor for v1 Codec.
var NewCodecV1 = apicodec.NewCodecV1

// NewCodecV2 is a constructor for v2 Codec.
var NewCodecV2 = apicodec.NewCodecV2

// Codec is responsible for encode/decode requests.
type Codec = apicodec.Codec

// DecodeKey is used to split a given key to it's APIv2 prefix and actual key.
var DecodeKey = apicodec.DecodeKey

// DefaultKeyspaceID is the keyspaceID of the default keyspace.
var DefaultKeyspaceID = apicodec.DefaultKeyspaceID

// DefaultKeyspaceName is the name of the default keyspace.
var DefaultKeyspaceName = apicodec.DefaultKeyspaceName

// Mode represents the operation mode of a request, export client.Mode
type Mode = apicodec.Mode

const (
	// ModeRaw represent a raw operation in TiKV, export client.ModeRaw
	ModeRaw Mode = apicodec.ModeRaw

	// ModeTxn represent a transaction operation in TiKV, export client.ModeTxn
	ModeTxn Mode = apicodec.ModeTxn
)

// KeyspaceID denotes the target keyspace of the request.
type KeyspaceID = apicodec.KeyspaceID

const (
	// NullspaceID is a special keyspace id that represents no keyspace exist
	NullspaceID KeyspaceID = apicodec.NullspaceID
)

// Store contains a kv process's address.
type Store = locate.Store

// Region presents kv region
type Region = locate.Region

// EpochNotMatch indicates it's invalidated due to epoch not match
const EpochNotMatch = locate.EpochNotMatch

// NewRPCanceller creates RPCCanceller with init state.
func NewRPCanceller() *RPCCanceller {
	return locate.NewRPCanceller()
}

// NewRegionVerID creates a region ver id, which used for invalidating regions.
func NewRegionVerID(id, confVer, ver uint64) RegionVerID {
	return locate.NewRegionVerID(id, confVer, ver)
}

// GetStoreTypeByMeta gets store type by store meta pb.
func GetStoreTypeByMeta(store *metapb.Store) tikvrpc.EndpointType {
	return tikvrpc.GetStoreTypeByMeta(store)
}

// NewRegionRequestSender creates a new sender.
func NewRegionRequestSender(regionCache *RegionCache, client client.Client, readTSValidator oracle.ReadTSValidator) *RegionRequestSender {
	return locate.NewRegionRequestSender(regionCache, client, readTSValidator)
}

// LoadShuttingDown atomically loads ShuttingDown.
func LoadShuttingDown() uint32 {
	return locate.LoadShuttingDown()
}

// StoreShuttingDown atomically stores ShuttingDown into v.
func StoreShuttingDown(v uint32) {
	locate.StoreShuttingDown(v)
}

// WithMatchLabels indicates selecting stores with matched labels
func WithMatchLabels(labels []*metapb.StoreLabel) StoreSelectorOption {
	return locate.WithMatchLabels(labels)
}

// WithMatchStores indicates selecting stores with matched store ids.
func WithMatchStores(stores []uint64) StoreSelectorOption {
	return locate.WithMatchStores(stores)
}

// NewRegionRequestRuntimeStats returns a new RegionRequestRuntimeStats.
func NewRegionRequestRuntimeStats() *RegionRequestRuntimeStats {
	return locate.NewRegionRequestRuntimeStats()
}

// SetRegionCacheTTLSec sets the base value of region cache TTL.
// Deprecated: use SetRegionCacheTTLWithJitter instead.
func SetRegionCacheTTLSec(t int64) {
	locate.SetRegionCacheTTLSec(t)
}

// ChangePDRegionMetaCircuitBreakerSettings changes circuit breaker settings for region metadata calls
func ChangePDRegionMetaCircuitBreakerSettings(apply func(config *circuitbreaker.Settings)) {
	locate.ChangePDRegionMetaCircuitBreakerSettings(apply)
}

// SetRegionCacheTTLWithJitter sets region cache TTL with jitter. The real TTL is in range of [base, base+jitter).
func SetRegionCacheTTLWithJitter(base int64, jitter int64) {
	locate.SetRegionCacheTTLWithJitter(base, jitter)
}

// SetStoreLivenessTimeout sets storeLivenessTimeout to t.
func SetStoreLivenessTimeout(t time.Duration) {
	locate.SetStoreLivenessTimeout(t)
}

// NewRegionCache creates a RegionCache.
func NewRegionCache(pdClient pd.Client) *locate.RegionCache {
	return locate.NewRegionCache(pdClient)
}

// LabelFilter returns false means label doesn't match, and will ignore this store.
type LabelFilter = locate.LabelFilter

// LabelFilterOnlyTiFlashWriteNode will only select stores whose label contains: <engine, tiflash> and <engine_role, write>.
// Only used for tiflash_compute node.
var LabelFilterOnlyTiFlashWriteNode = locate.LabelFilterOnlyTiFlashWriteNode

// LabelFilterNoTiFlashWriteNode will only select stores whose label contains: <engine, tiflash>, but not contains <engine_role, write>.
// Normally tidb use this filter.
var LabelFilterNoTiFlashWriteNode = locate.LabelFilterNoTiFlashWriteNode

// LabelFilterAllTiFlashNode will select all tiflash stores.
var LabelFilterAllTiFlashNode = locate.LabelFilterAllTiFlashNode

// LabelFilterAllNode will select all stores.
var LabelFilterAllNode = locate.LabelFilterAllNode

// KeyRange represents a range where StartKey <= key < EndKey.
type KeyRange = kv.KeyRange

// BatchLocateKeyRangesOpt is the option for BatchLocateKeyRanges.
type BatchLocateKeyRangesOpt = locate.BatchLocateKeyRangesOpt

var (
	// WithNeedBuckets indicates that the request needs to contain bucket info.
	WithNeedBuckets = locate.WithNeedBuckets
	// WithNeedRegionHasLeaderPeer indicates that the regions returned must contain leader peer, unless it's skipped.
	// Note the leader peer existence is not guaranteed is not related to the election status,
	// the region info contains old leader during the election, this variable affects nothing in most time.
	WithNeedRegionHasLeaderPeer = locate.WithNeedRegionHasLeaderPeer
)
