package queue

import (
	"encoding/json"
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- mockChannel ----

type mockChannel struct {
	declareErr    error
	publishErr    error
	failOnDeclare int // fail when declareCount reaches this (1-indexed, 0 = never fail)
	declareCount  int
	published     []amqp.Publishing
}

func (m *mockChannel) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	m.declareCount++
	if m.failOnDeclare > 0 && m.declareCount == m.failOnDeclare {
		return amqp.Queue{}, m.declareErr
	}
	return amqp.Queue{Name: name}, nil
}

func (m *mockChannel) Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	m.published = append(m.published, msg)
	return m.publishErr
}

// ---- NewProducer tests ----

func TestNewProducer_DeclareError_Queue1(t *testing.T) {
	ch := &mockChannel{declareErr: errors.New("fail"), failOnDeclare: 1}
	_, err := NewProducer(ch)
	require.Error(t, err)
}

func TestNewProducer_DeclareError_Queue2(t *testing.T) {
	ch := &mockChannel{declareErr: errors.New("fail"), failOnDeclare: 2}
	_, err := NewProducer(ch)
	require.Error(t, err)
}

func TestNewProducer_DeclareError_Queue3(t *testing.T) {
	ch := &mockChannel{declareErr: errors.New("fail"), failOnDeclare: 3}
	_, err := NewProducer(ch)
	require.Error(t, err)
}

func TestNewProducer_DeclareError_Queue4(t *testing.T) {
	ch := &mockChannel{declareErr: errors.New("fail"), failOnDeclare: 4}
	_, err := NewProducer(ch)
	require.Error(t, err)
}

func TestNewProducer_DeclareError_Queue5(t *testing.T) {
	ch := &mockChannel{declareErr: errors.New("fail"), failOnDeclare: 5}
	_, err := NewProducer(ch)
	require.Error(t, err)
}

func TestNewProducer_DeclareError_Queue6(t *testing.T) {
	ch := &mockChannel{declareErr: errors.New("fail"), failOnDeclare: 6}
	_, err := NewProducer(ch)
	require.Error(t, err)
}

func TestNewProducer_Success(t *testing.T) {
	ch := &mockChannel{}
	p, err := NewProducer(ch)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

// ---- Publish tests ----

func TestPublish_Success(t *testing.T) {
	ch := &mockChannel{}
	p, _ := NewProducer(ch)
	err := p.Publish(ActivationMessage{Email: "a@b.com", FirstName: "X", ActivationLink: "http://l"})
	require.NoError(t, err)
	require.Len(t, ch.published, 1)
	assert.Equal(t, "application/json", ch.published[0].ContentType)
}

func TestPublish_Error(t *testing.T) {
	ch := &mockChannel{publishErr: errors.New("publish failed")}
	p, _ := NewProducer(ch)
	err := p.Publish(ActivationMessage{Email: "a@b.com"})
	require.Error(t, err)
}

func TestPublishPasswordReset_Success(t *testing.T) {
	ch := &mockChannel{}
	p, _ := NewProducer(ch)
	err := p.PublishPasswordReset(PasswordResetMessage{Email: "a@b.com", FirstName: "X", ResetLink: "http://r"})
	require.NoError(t, err)
	require.Len(t, ch.published, 1)
}

func TestPublishPasswordReset_Error(t *testing.T) {
	ch := &mockChannel{publishErr: errors.New("fail")}
	p, _ := NewProducer(ch)
	err := p.PublishPasswordReset(PasswordResetMessage{Email: "a@b.com"})
	require.Error(t, err)
}

func TestPublishPasswordConfirmation_Success(t *testing.T) {
	ch := &mockChannel{}
	p, _ := NewProducer(ch)
	err := p.PublishPasswordConfirmation(PasswordConfirmationMessage{Email: "a@b.com", FirstName: "X"})
	require.NoError(t, err)
	require.Len(t, ch.published, 1)
}

func TestPublishPasswordConfirmation_Error(t *testing.T) {
	ch := &mockChannel{publishErr: errors.New("fail")}
	p, _ := NewProducer(ch)
	err := p.PublishPasswordConfirmation(PasswordConfirmationMessage{Email: "a@b.com"})
	require.Error(t, err)
}

func TestPublishAccountCreated_Success(t *testing.T) {
	ch := &mockChannel{}
	p, _ := NewProducer(ch)
	err := p.PublishAccountCreated(AccountCreatedMessage{Email: "a@b.com", FirstName: "X", AccountName: "N", AccountNumber: "123", CurrencyCode: "RSD"})
	require.NoError(t, err)
	require.Len(t, ch.published, 1)
}

func TestPublishAccountCreated_Error(t *testing.T) {
	ch := &mockChannel{publishErr: errors.New("fail")}
	p, _ := NewProducer(ch)
	err := p.PublishAccountCreated(AccountCreatedMessage{Email: "a@b.com"})
	require.Error(t, err)
}

func TestPublishCardConfirmation_Success(t *testing.T) {
	ch := &mockChannel{}
	p, _ := NewProducer(ch)
	err := p.PublishCardConfirmation(CardConfirmationMessage{Email: "a@b.com", FirstName: "X", ConfirmationCode: "123456"})
	require.NoError(t, err)
	require.Len(t, ch.published, 1)
}

func TestPublishCardConfirmation_Error(t *testing.T) {
	ch := &mockChannel{publishErr: errors.New("fail")}
	p, _ := NewProducer(ch)
	err := p.PublishCardConfirmation(CardConfirmationMessage{Email: "a@b.com"})
	require.Error(t, err)
}

func TestPublishLoanLatePayment_Success(t *testing.T) {
	ch := &mockChannel{}
	p, _ := NewProducer(ch)
	err := p.PublishLoanLatePayment(LoanLatePaymentMessage{Email: "a@b.com", FirstName: "X", LoanNumber: "1", AmountDue: 100.0, Currency: "RSD", RetryCount: 1})
	require.NoError(t, err)
	require.Len(t, ch.published, 1)
}

func TestPublishLoanLatePayment_Error(t *testing.T) {
	ch := &mockChannel{publishErr: errors.New("fail")}
	p, _ := NewProducer(ch)
	err := p.PublishLoanLatePayment(LoanLatePaymentMessage{Email: "a@b.com"})
	require.Error(t, err)
}

func TestActivationMessage_JSON(t *testing.T) {
	orig := ActivationMessage{
		Email:          "user@example.com",
		FirstName:      "John",
		ActivationLink: "http://example.com/activate/abc123",
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded ActivationMessage
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig, decoded)
}

func TestActivationMessage_JSONKeys(t *testing.T) {
	msg := ActivationMessage{Email: "a@b.com", FirstName: "X", ActivationLink: "http://link"}
	data, _ := json.Marshal(msg)

	var raw map[string]string
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Contains(t, raw, "email")
	assert.Contains(t, raw, "first_name")
	assert.Contains(t, raw, "activation_link")
}

func TestPasswordResetMessage_JSON(t *testing.T) {
	orig := PasswordResetMessage{
		Email:     "user@example.com",
		FirstName: "Jane",
		ResetLink: "http://example.com/reset/xyz",
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded PasswordResetMessage
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig, decoded)
}

func TestPasswordResetMessage_JSONKeys(t *testing.T) {
	msg := PasswordResetMessage{Email: "a@b.com", FirstName: "X", ResetLink: "http://link"}
	data, _ := json.Marshal(msg)

	var raw map[string]string
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Contains(t, raw, "email")
	assert.Contains(t, raw, "first_name")
	assert.Contains(t, raw, "reset_link")
}

func TestPasswordConfirmationMessage_JSON(t *testing.T) {
	orig := PasswordConfirmationMessage{
		Email:     "user@example.com",
		FirstName: "Bob",
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded PasswordConfirmationMessage
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig, decoded)
}

func TestAccountCreatedMessage_JSON(t *testing.T) {
	orig := AccountCreatedMessage{
		Email:         "user@example.com",
		FirstName:     "Ana",
		AccountName:   "Tekući račun",
		AccountNumber: "265000191399797801",
		CurrencyCode:  "RSD",
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded AccountCreatedMessage
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig, decoded)
}

func TestAccountCreatedMessage_JSONKeys(t *testing.T) {
	msg := AccountCreatedMessage{Email: "a@b.com", FirstName: "X", AccountName: "N", AccountNumber: "123", CurrencyCode: "RSD"}
	data, _ := json.Marshal(msg)

	var raw map[string]string
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Contains(t, raw, "email")
	assert.Contains(t, raw, "first_name")
	assert.Contains(t, raw, "account_name")
	assert.Contains(t, raw, "account_number")
	assert.Contains(t, raw, "currency_code")
}

func TestCardConfirmationMessage_JSON(t *testing.T) {
	orig := CardConfirmationMessage{
		Email:            "user@example.com",
		FirstName:        "Marko",
		ConfirmationCode: "482019",
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded CardConfirmationMessage
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig, decoded)
}

func TestCardConfirmationMessage_JSONKeys(t *testing.T) {
	msg := CardConfirmationMessage{Email: "a@b.com", FirstName: "X", ConfirmationCode: "123456"}
	data, _ := json.Marshal(msg)

	var raw map[string]string
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Contains(t, raw, "email")
	assert.Contains(t, raw, "first_name")
	assert.Contains(t, raw, "confirmation_code")
}

func TestLoanLatePaymentMessage_JSON(t *testing.T) {
	orig := LoanLatePaymentMessage{
		Email:      "user@example.com",
		FirstName:  "Petar",
		LoanNumber: "1234567890123",
		AmountDue:  12500.50,
		Currency:   "RSD",
		RetryCount: 2,
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded LoanLatePaymentMessage
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig, decoded)
}

func TestLoanLatePaymentMessage_JSONKeys(t *testing.T) {
	msg := LoanLatePaymentMessage{Email: "a@b.com", FirstName: "X", LoanNumber: "123", AmountDue: 100.0, Currency: "RSD", RetryCount: 1}
	data, _ := json.Marshal(msg)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Contains(t, raw, "email")
	assert.Contains(t, raw, "first_name")
	assert.Contains(t, raw, "loan_number")
	assert.Contains(t, raw, "amount_due")
	assert.Contains(t, raw, "currency")
	assert.Contains(t, raw, "retry_count")
}
