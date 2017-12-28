package data

type Balance struct {
	PublicKey string
	Value     int64
}

type IBalanceRepository interface {
	Get(publicKey string) (*Balance, error)
	Insert(balance *Balance) error
	Update(balance *Balance) error
}