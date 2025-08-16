package main

import (
    "encoding/json"
    "fmt"

    "github.com/hyperledger/fabric-contract-api-go/contractapi"
)

// Account represents the asset structure
type Account struct {
    DealerID    string  `json:"dealerId"`
    MSISDN      string  `json:"msisdn"`
    MPIN        string  `json:"mpin"`
    Balance     float64 `json:"balance"`
    Status      string  `json:"status"`
    TransAmount float64 `json:"transAmount"`
    TransType   string  `json:"transType"`
    Remarks     string  `json:"remarks"`
}

// AccountContract provides functions for managing Accounts
type AccountContract struct {
    contractapi.Contract
}

// InitLedger
func (ac *AccountContract) InitLedger(ctx contractapi.TransactionContextInterface) error {
    // create a few sample accounts if desired
    accounts := []Account{
        {DealerID: "A001", MSISDN: "9998887701", MPIN: "1111", Balance: 1000, Status: "active", TransAmount: 500, TransType: "credit", Remarks: "init"},
        {DealerID: "A002", MSISDN: "9998887702", MPIN: "2222", Balance: 2000, Status: "active", TransAmount: 1000, TransType: "credit", Remarks: "init"},
    }
    for _, a := range accounts {
        b, _ := json.Marshal(a)
        _ = ctx.GetStub().PutState(a.DealerID, b)
    }
    return nil
}

// AccountExists returns true when account with given id exists in world state
func (ac *AccountContract) AccountExists(ctx contractapi.TransactionContextInterface, dealerId string) (bool, error) {
    data, err := ctx.GetStub().GetState(dealerId)
    if err != nil {
        return false, err
    }
    return data != nil, nil
}

// CreateAccount creates a new account in the ledger
func (ac *AccountContract) CreateAccount(ctx contractapi.TransactionContextInterface, dealerId, msisdn, mpin string, balanceStr, status, transAmountStr, transType, remarks string) error {
    exists, err := ac.AccountExists(ctx, dealerId)
    if err != nil {
        return err
    }
    if exists {
        return fmt.Errorf("account %s already exists", dealerId)
    }

    // convert numeric fields
    var balance float64
    var transAmount float64
    // basic conversion without heavy error handling for brevity
    fmt.Sscanf(balanceStr, "%f", &balance)
    fmt.Sscanf(transAmountStr, "%f", &transAmount)

    account := Account{
        DealerID:    dealerId,
        MSISDN:      msisdn,
        MPIN:        mpin,
        Balance:     balance,
        Status:      status,
        TransAmount: transAmount,
        TransType:   transType,
        Remarks:     remarks,
    }
    bytes, err := json.Marshal(account)
    if err != nil {
        return err
    }
    return ctx.GetStub().PutState(dealerId, bytes)
}

// QueryAccount returns the account stored in the world state with given id
func (ac *AccountContract) QueryAccount(ctx contractapi.TransactionContextInterface, dealerId string) (*Account, error) {
    data, err := ctx.GetStub().GetState(dealerId)
    if err != nil {
        return nil, fmt.Errorf("failed to read from world state: %v", err)
    }
    if data == nil {
        return nil, fmt.Errorf("account %s does not exist", dealerId)
    }
    var account Account
    if err := json.Unmarshal(data, &account); err != nil {
        return nil, err
    }
    return &account, nil
}

// UpdateAccountBalance updates balance and records the transaction fields
func (ac *AccountContract) UpdateAccountBalance(ctx contractapi.TransactionContextInterface, dealerId, balanceStr string) error {
    data, err := ctx.GetStub().GetState(dealerId)
    if err != nil {
        return err
    }
    if data == nil {
        return fmt.Errorf("account %s does not exist", dealerId)
    }
    var account Account
    if err := json.Unmarshal(data, &account); err != nil {
        return err
    }

    var balance float64
    fmt.Sscanf(balanceStr, "%f", &balance)
    account.Balance = balance

    bytes, err := json.Marshal(account)
    if err != nil {
        return err
    }
    return ctx.GetStub().PutState(dealerId, bytes)
}

// GetAccountHistory returns the history of a key as a slice of states
func (ac *AccountContract) GetAccountHistory(ctx contractapi.TransactionContextInterface, dealerId string) ([]map[string]interface{}, error) {
    iter, err := ctx.GetStub().GetHistoryForKey(dealerId)
    if err != nil {
        return nil, err
    }
    defer iter.Close()

    var history []map[string]interface{}
    for iter.HasNext() {
        resp, err := iter.Next()
        if err != nil {
            return nil, err
        }
        var acc Account
        _ = json.Unmarshal(resp.Value, &acc)
        entry := map[string]interface{}{
            "TxId":      resp.TxId,
            "Timestamp": resp.Timestamp,
            "IsDelete":  resp.IsDelete,
            "Value":     acc,
        }
        history = append(history, entry)
    }
    return history, nil
}

func main() {
    chaincode, err := contractapi.NewChaincode(new(AccountContract))
    if err != nil {
        fmt.Printf("Error create account chaincode: %s", err.Error())
        return
    }
    if err := chaincode.Start(); err != nil {
        fmt.Printf("Error starting chaincode: %s", err.Error())
    }
}
