package models

type StockExchange struct {
	ID       int64
	Name     string
	Acronym  string
	MICCode  string
	Polity   string
	Currency string
	Timezone string
}
