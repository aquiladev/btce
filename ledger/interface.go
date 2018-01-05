package ledger

type Config struct {
	BatchSize            int
	BalanceCalcPeriod    int32
	BalanceCalcThreshold int32
}