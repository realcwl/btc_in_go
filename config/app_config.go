package config

// This is the global app config for the blockchain.
type AppConfig struct {
	// How many leading 0s to form a valid hash.
	DIFFICULTY int
	// The default coinbase reward.
	COINBASE_REWARD float64
}
