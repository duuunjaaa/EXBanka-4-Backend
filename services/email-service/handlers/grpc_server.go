package handlers

import (
	"context"
	"net/mail"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/RAF-SI-2025/EXBanka-4-Backend/services/email-service/queue"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/email"
)

type Publisher interface {
	Publish(msg queue.ActivationMessage) error
	PublishPasswordReset(msg queue.PasswordResetMessage) error
	PublishPasswordConfirmation(msg queue.PasswordConfirmationMessage) error
	PublishAccountCreated(msg queue.AccountCreatedMessage) error
	PublishCardConfirmation(msg queue.CardConfirmationMessage) error
	PublishLoanLatePayment(msg queue.LoanLatePaymentMessage) error
}

type EmailServer struct {
	pb.UnimplementedEmailServiceServer
	Producer Publisher
}

func (s *EmailServer) SendActivationEmail(_ context.Context, req *pb.SendActivationEmailRequest) (*pb.SendActivationEmailResponse, error) {
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid email address: %v", err)
	}
	err := s.Producer.Publish(queue.ActivationMessage{
		Email:          req.Email,
		FirstName:      req.FirstName,
		ActivationLink: req.ActivationLink,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to enqueue email: %v", err)
	}
	return &pb.SendActivationEmailResponse{}, nil
}

func (s *EmailServer) SendPasswordResetEmail(_ context.Context, req *pb.SendPasswordResetEmailRequest) (*pb.SendPasswordResetEmailResponse, error) {
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid email address: %v", err)
	}
	err := s.Producer.PublishPasswordReset(queue.PasswordResetMessage{
		Email:     req.Email,
		FirstName: req.FirstName,
		ResetLink: req.ResetLink,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to enqueue email: %v", err)
	}
	return &pb.SendPasswordResetEmailResponse{}, nil
}

func (s *EmailServer) SendAccountCreatedEmail(_ context.Context, req *pb.SendAccountCreatedEmailRequest) (*pb.SendAccountCreatedEmailResponse, error) {
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid email address: %v", err)
	}
	err := s.Producer.PublishAccountCreated(queue.AccountCreatedMessage{
		Email:         req.Email,
		FirstName:     req.FirstName,
		AccountName:   req.AccountName,
		AccountNumber: req.AccountNumber,
		CurrencyCode:  req.CurrencyCode,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to enqueue email: %v", err)
	}
	return &pb.SendAccountCreatedEmailResponse{}, nil
}

func (s *EmailServer) SendPasswordConfirmationEmail(_ context.Context, req *pb.SendActivationEmailRequest) (*pb.SendActivationEmailResponse, error) {
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid email address: %v", err)
	}
	err := s.Producer.PublishPasswordConfirmation(queue.PasswordConfirmationMessage{
		Email:     req.Email,
		FirstName: req.FirstName,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to enqueue email: %v", err)
	}
	return &pb.SendActivationEmailResponse{}, nil
}

func (s *EmailServer) SendCardConfirmationEmail(_ context.Context, req *pb.SendCardConfirmationEmailRequest) (*pb.SendCardConfirmationEmailResponse, error) {
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid email address: %v", err)
	}
	err := s.Producer.PublishCardConfirmation(queue.CardConfirmationMessage{
		Email:            req.Email,
		FirstName:        req.FirstName,
		ConfirmationCode: req.ConfirmationCode,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to enqueue email: %v", err)
	}
	return &pb.SendCardConfirmationEmailResponse{}, nil
}

func (s *EmailServer) SendLoanLatePaymentEmail(_ context.Context, req *pb.SendLoanLatePaymentEmailRequest) (*pb.SendLoanLatePaymentEmailResponse, error) {
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid email address: %v", err)
	}
	err := s.Producer.PublishLoanLatePayment(queue.LoanLatePaymentMessage{
		Email:      req.Email,
		FirstName:  req.FirstName,
		LoanNumber: req.LoanNumber,
		AmountDue:  req.AmountDue,
		Currency:   req.Currency,
		RetryCount: req.RetryCount,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to enqueue email: %v", err)
	}
	return &pb.SendLoanLatePaymentEmailResponse{}, nil
}
