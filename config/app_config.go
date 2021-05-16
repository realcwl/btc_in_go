package config

// This is the global app config for the blockchain.
type AppConfig struct {
	// How many leading 0s to form a valid hash.
	DIFFICULTY int `yaml:"DIFFICULTY"`
	// The default coinbase reward.
	COINBASE_REWARD float64 `yaml:"COINBASE_REWARD"`
	// How deep a block is confirmed. Aka how many block need to be after this block to confirm a block.
	CONFIRMATION int64 `yaml:"CONFIRMATION"`
	// Whether or not to remine the block if tail changed in between.
	REMINE_ON_TAIL_CHANGE bool `yaml:"REMINE_ON_TAIL_CHANGE"`
}
