package ledger

type BalanceCalc struct {
	db DB
}

func (bc *BalanceCalc) Calc() error {
	log.Info("Recalculating balance")

	for _, k := range bc.fetchKeys() {
		entries, err := bc.getEntries(k)
		if err != nil {
			return err
		}

		value := int64(0)
		for _, entry := range entries {
			if entry.In {
				value += entry.Value
			} else {
				value -= entry.Value
			}
		}
		err = bc.db.PutBalance(&Balance{
			Key:   []byte(k),
			Value: value,
		})
		if err != nil {
			return err
		}
	}
	log.Info("Balance recalculated")

	return nil
}

func (bc *BalanceCalc) getEntries(key string) ([]Entry, error) {
	var entries []Entry

	iterator := bc.db.GetIterator([]byte(key))
	for iterator.Next() {
		entry, err := ToEntry(iterator.Key(), iterator.Value())
		if err != nil {
			return nil, err
		}
		entries = append(entries, *entry)
	}
	iterator.Release()

	return entries, nil
}

func (bc *BalanceCalc) fetchKeys() []string {
	var keys []string

	iterator := bc.db.GetIterator(addressIdxBucketName)
	for iterator.Next() {
		keys = append(keys, string(iterator.Value()))
	}
	iterator.Release()

	return keys
}

func newBalanceCalc(db DB) *BalanceCalc {
	return &BalanceCalc{db: db}
}
