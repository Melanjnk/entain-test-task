package service

// ProcessTransactionCommand carries raw input for the process-transaction use case.
type ProcessTransactionCommand struct {
	UserID        uint64
	SourceType    string
	State         string
	Amount        string
	TransactionID string
}
