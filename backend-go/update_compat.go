package main

import (
	"context"
	"errors"

	updatepkg "fund-tracking-backend-go/internal/update"
)

var fundUpdateService *updatepkg.Service

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func isValidFundCode(code string) bool {
	return updatepkg.IsValidFundCode(code)
}

func buildUpdateFundCodes(ctx context.Context) ([]string, []string, error) {
	if fundUpdateService == nil {
		return nil, nil, errors.New("fund update service is not initialized")
	}
	return fundUpdateService.BuildUpdateFundCodes(ctx)
}

func fetchFundBasicInfo(ctx context.Context, fundCode string) (Fund, error) {
	if fundUpdateService == nil {
		return Fund{}, errors.New("fund update service is not initialized")
	}
	fund, err := fundUpdateService.FetchFundBasicInfo(ctx, fundCode)
	if err != nil {
		return Fund{}, err
	}
	return fundFromUpdatePackage(fund), nil
}

func validateFetchedFund(requestedCode string, fund Fund) error {
	return updatepkg.ValidateFetchedFund(requestedCode, fundToUpdatePackage(fund))
}

func upsertFundBasicInfo(ctx context.Context, fund Fund) error {
	if fundUpdateService == nil {
		return errors.New("fund update service is not initialized")
	}
	return fundUpdateService.UpsertFundBasicInfo(ctx, fundToUpdatePackage(fund))
}

func fundFromUpdatePackage(fund updatepkg.Fund) Fund {
	return Fund{
		FundCode:     fund.FundCode,
		FundName:     fund.FundName,
		NetValue:     fund.NetValue,
		DayGrowth:    fund.DayGrowth,
		NetValueDate: fund.NetValueDate,
		UpdateTime:   fund.UpdateTime,
		IsSeed:       fund.IsSeed,
	}
}

func fundToUpdatePackage(fund Fund) updatepkg.Fund {
	return updatepkg.Fund{
		FundCode:     fund.FundCode,
		FundName:     fund.FundName,
		NetValue:     fund.NetValue,
		DayGrowth:    fund.DayGrowth,
		NetValueDate: fund.NetValueDate,
		UpdateTime:   fund.UpdateTime,
		IsSeed:       fund.IsSeed,
	}
}
