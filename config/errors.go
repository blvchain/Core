package config

import "errors"

var (
	// DB errors
	ErrCanNotConnectToMongoDB   = errors.New("xd01: Cannot connect to mongodb")
	ErrCanNotGetPingFromMongoDB = errors.New("xd02: Cannot get ping from mongodb")
	ErrDuplicateKey             = errors.New("xd03: Duplicate key error")
	ErrInsertingToDB            = errors.New("xd04: Cannot insert data to mongodb")
	ErrCursor                   = errors.New("xd05: Cursor error in find many")
	ErrFindMany                 = errors.New("xd06: Find many error in mongodb")
	ErrNoDNSSeed                = errors.New("xd07: There is no DNS seed in DB")

	// Request errors
	ErrBindQuery            = errors.New("xr01: Cannot bind query")
	ErrBindBody             = errors.New("xr01: Cannot bind body")
	ErrAddressIsNotValid    = errors.New("xr01: Wallet addrress is not valid")
	ErrNoQuery              = errors.New("xr02: This request must has at least one query key,value")
	ErrTooManyRequests      = errors.New("xr03: Too many requests, please try again later")
	ErrMinerTooManyRequests = errors.New("xr03: Too many requests, if you want to use several miners, please add diffrent wallet for eachone")
	ErrSenderLowBalance     = errors.New("xr04: Sender balance dose not support minimum balance to send funds")
	ErrDuplicatedData       = errors.New("xr05: A Data with this signature is already exists")
	ErrNotValidSignature    = errors.New("xr06: Signature is not valid")
	ErrNoValidBlock         = errors.New("xr06: Cannot find a valid block")
	ErrTokenExpired         = errors.New("xr06: Token expired")
	ErrPuzzleNotSolved      = errors.New("xr06: Puzzle not solved")
	ErrAuthNeeded           = errors.New("xr06: Auth needed")
	ErrWrongAuthElemets     = errors.New("xr06: Auth elements is not correct")
	ErrWrongAuth            = errors.New("xr06: Puzzle is not correct")
	ErrInvalidToken         = errors.New("xr06: Token is invalid")
	ErrExpiredToken         = errors.New("xr06: Token is expired. Need new auth")
	ErrDoubleAuth           = errors.New("xr06: Double solving the puzzle. Need for new auth")

	// Server errors
	ErrInternalServer = errors.New("xs01: Internal server error")

	// Function errors
	ErrNotValidPublicKey = errors.New("invalid public key")
	ErrBlockCapacity     = errors.New("block capacity is full")
)
