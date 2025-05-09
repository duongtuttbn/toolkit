package model

type TokenInfo struct {
	TokenAddress     string  `json:"token_address"`
	TokenName        string  `json:"token_name"`
	TokenSymbol      string  `json:"token_symbol"`
	ContractDecimals int64   `json:"contract_decimals"`
	TotalSupply      float64 `json:"total_supply"`
}

type LiquidityPoolInfo struct {
	PoolAddress string `json:"pool_address"`
	Token0      string `json:"token0"`
	Token1      string `json:"token1"`
}
