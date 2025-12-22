// Package fields provides blockchain-specific logging field helpers.
//
// These helpers create structured fields with consistent naming for
// blockchain-related data like transactions, blocks, shards, and timing.
//
// Usage:
//
//	import "MRE/pkg/ion/fields"
//
//	logger.Info("transaction routed",
//	    fields.TxHash("abc123"),
//	    fields.ShardID(5),
//	    fields.LatencyMs(12.5),
//	)
package fields

import "MRE/pkg/ion"

// --- Transaction Fields ---

// TxHash creates a transaction hash field.
func TxHash(hash string) ion.Field {
	return ion.String("tx_hash", hash)
}

// TxSignature creates a transaction signature field.
func TxSignature(sig string) ion.Field {
	return ion.String("tx_signature", sig)
}

// TxStatus creates a transaction status field.
func TxStatus(status string) ion.Field {
	return ion.String("tx_status", status)
}

// TxType creates a transaction type field.
func TxType(txType string) ion.Field {
	return ion.String("tx_type", txType)
}

// --- Block Fields ---

// BlockHeight creates a block height field.
func BlockHeight(height uint64) ion.Field {
	return ion.Field{Key: "block_height", Value: height}
}

// BlockHash creates a block hash field.
func BlockHash(hash string) ion.Field {
	return ion.String("block_hash", hash)
}

// Slot creates a slot number field (Solana).
func Slot(slot uint64) ion.Field {
	return ion.Field{Key: "slot", Value: slot}
}

// Epoch creates an epoch field.
func Epoch(epoch uint64) ion.Field {
	return ion.Field{Key: "epoch", Value: epoch}
}

// --- Chain Context Fields ---

// ChainID creates a chain ID field.
func ChainID(id string) ion.Field {
	return ion.String("chain_id", id)
}

// Network creates a network field (mainnet, devnet, testnet).
func Network(net string) ion.Field {
	return ion.String("network", net)
}

// ShardID creates a shard ID field.
func ShardID(id int) ion.Field {
	return ion.Int("shard_id", id)
}

// NodeID creates a node ID field.
func NodeID(id string) ion.Field {
	return ion.String("node_id", id)
}

// Address creates an address field (wallet/contract).
func Address(addr string) ion.Field {
	return ion.String("address", addr)
}

// --- Timing Fields ---

// LatencyMs creates a latency field in milliseconds.
func LatencyMs(ms float64) ion.Field {
	return ion.Float64("latency_ms", ms)
}

// DurationMs creates a duration field in milliseconds.
func DurationMs(ms float64) ion.Field {
	return ion.Float64("duration_ms", ms)
}

// DurationSec creates a duration field in seconds.
func DurationSec(sec float64) ion.Field {
	return ion.Float64("duration_sec", sec)
}

// --- Count/Size Fields ---

// Count creates a generic count field.
func Count(n int) ion.Field {
	return ion.Int("count", n)
}

// Size creates a size field in bytes.
func Size(bytes int64) ion.Field {
	return ion.Int64("size_bytes", bytes)
}

// Pending creates a pending count field.
func Pending(n int) ion.Field {
	return ion.Int("pending_count", n)
}

// Total creates a total count field.
func Total(n int) ion.Field {
	return ion.Int("total", n)
}

// --- Component Fields ---

// Component creates a component name field.
func Component(name string) ion.Field {
	return ion.String("component", name)
}

// Operation creates an operation name field.
func Operation(op string) ion.Field {
	return ion.String("operation", op)
}

// Method creates a method name field (gRPC/HTTP).
func Method(method string) ion.Field {
	return ion.String("method", method)
}

// --- Connection Fields ---

// Host creates a host field.
func Host(host string) ion.Field {
	return ion.String("host", host)
}

// Port creates a port field.
func Port(port int) ion.Field {
	return ion.Int("port", port)
}

// RemoteAddr creates a remote address field.
func RemoteAddr(addr string) ion.Field {
	return ion.String("remote_addr", addr)
}

// --- Weight/Score Fields (MRE-specific) ---

// Weight creates a weight field.
func Weight(w float64) ion.Field {
	return ion.Float64("weight", w)
}

// Score creates a score field.
func Score(s float64) ion.Field {
	return ion.Float64("score", s)
}

// ReplicaIndex creates a replica index field.
func ReplicaIndex(idx int) ion.Field {
	return ion.Int("replica_index", idx)
}

// --- Status Fields ---

// Success creates a success boolean field.
func Success(ok bool) ion.Field {
	return ion.Bool("success", ok)
}

// Enabled creates an enabled boolean field.
func Enabled(on bool) ion.Field {
	return ion.Bool("enabled", on)
}

// Reason creates a reason field (for failures/decisions).
func Reason(r string) ion.Field {
	return ion.String("reason", r)
}
