package types

import "cosmossdk.io/errors"

var (
	ErrDenomExists        = errors.Register(ModuleName, 2, "denom already exists")
	ErrDenomNotFound      = errors.Register(ModuleName, 3, "denom policy not found")
	ErrSupplyCapExceeded  = errors.Register(ModuleName, 4, "mint would exceed capped supply")
	ErrInvalidSupplyMode      = errors.Register(ModuleName, 5, "invalid supply mode")
	ErrInvalidSubdenom        = errors.Register(ModuleName, 6, "invalid subdenom")
	ErrBurnDisabled           = errors.Register(ModuleName, 7, "burn is disabled for this denom")
	ErrInvalidCollateral      = errors.Register(ModuleName, 8, "invalid collateral configuration")
	ErrInsufficientCollateral = errors.Register(ModuleName, 9, "insufficient collateral provided")
	ErrTokenPaused            = errors.Register(ModuleName, 10, "token transfers are paused")
	ErrTransferFeeExceeded    = errors.Register(ModuleName, 11, "transfer fee exceeds maximum allowed limit")
)
