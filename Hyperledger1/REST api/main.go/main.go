package main

import (
    "context"
    "crypto/x509"
    "encoding/pem"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "time"

    "github.com/gorilla/mux"
    "github.com/hyperledger/fabric-gateway/pkg/client"
    "github.com/hyperledger/fabric-gateway/pkg/client/identity"
    "github.com/hyperledger/fabric-gateway/pkg/client/signing"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
)

func newIdentity(certPath string) (*identity.X509Identity, error) {
    certPEM, err := ioutil.ReadFile(certPath)
    if err != nil {
        return nil, err
    }

    block, _ := pem.Decode(certPEM)
    if block == nil {
        return nil, fmt.Errorf("failed to decode PEM cert")
    }
    cert, err := x509.ParseCertificate(block.Bytes)
    if err != nil {
        return nil, err
    }

    id, err := identity.NewX509Identity("Org1MSP", cert)
    if err != nil {
        return nil, err
    }
    return id, nil
}

func newSign(keyPath string) (signing.Sign, error) {
    keyPEM, err := ioutil.ReadFile(keyPath)
    if err != nil {
        return nil, err
    }
    block, _ := pem.Decode(keyPEM)
    if block == nil {
        return nil, fmt.Errorf("failed to decode PEM key")
    }

    priv, err := signing.NewPrivateKey(block.Bytes)
    if err != nil {
        return nil, err
    }

    sign, err := signing.NewSigner(priv)
    if err != nil {
        return nil, err
    }
    return sign, nil
}

func main() {
    // Edit these paths to match your environment
    mspCert := os.Getenv("MSP_CERT_PATH")
    mspKey := os.Getenv("MSP_KEY_PATH")
    if mspCert == "" || mspKey == "" {
        log.Fatal("Please set MSP_CERT_PATH and MSP_KEY_PATH environment variables to your admin cert and key locations")
    }

    id, err := newIdentity(mspCert)
    if err != nil {
        log.Fatalf("Failed to create identity: %v", err)
    }

    sign, err := newSign(mspKey)
    if err != nil {
        log.Fatalf("Failed to create signer: %v", err)
    }

    // Create gRPC connection to peer (use TLS)
    tlsCertPath := os.Getenv("PEER_TLS_CERT")
    if tlsCertPath == "" {
        log.Fatal("Please set PEER_TLS_CERT environment variable to peer tls ca cert")
    }
    certPool := x509.NewCertPool()
    tlsPem, err := ioutil.ReadFile(tlsCertPath)
    if err != nil {
        log.Fatalf("Failed to read TLS cert: %v", err)
    }
    if ok := certPool.AppendCertsFromPEM(tlsPem); !ok {
        log.Fatalf("Failed to append peer TLS cert")
    }
    creds := credentials.NewClientTLSFromCert(certPool, "")
    conn, err := grpc.Dial("localhost:7051", grpc.WithTransportCredentials(creds))
    if err != nil {
        log.Fatalf("Failed to dial peer: %v", err)
    }
    defer conn.Close()

    gw, err := client.Connect(id, client.WithSign(sign), client.WithClientConnection(conn))
    if err != nil {
        log.Fatalf("Failed to connect to gateway: %v", err)
    }
    defer gw.Close()

    network := gw.GetNetwork("mychannel")
    contract := network.GetContract("accountcc")

    r := mux.NewRouter()

    r.HandleFunc("/accounts", func(w http.ResponseWriter, r *http.Request) {
        // create account
        dealerId := r.FormValue("dealerId")
        msisdn := r.FormValue("msisdn")
        mpin := r.FormValue("mpin")
        balance := r.FormValue("balance")
        status := r.FormValue("status")
        transAmount := r.FormValue("transAmount")
        transType := r.FormValue("transType")
        remarks := r.FormValue("remarks")

        // Submit transaction
        _, err := contract.SubmitTransaction("CreateAccount", dealerId, msisdn, mpin, balance, status, transAmount, transType, remarks)
        if err != nil {
            w.WriteHeader(http.StatusInternalServerError)
            fmt.Fprintf(w, "CreateAccount failed: %v", err)
            return
        }
        fmt.Fprintf(w, "Account %s created", dealerId)
    }).Methods("POST")

    r.HandleFunc("/accounts/{id}", func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        id := vars["id"]
        result, err := contract.EvaluateTransaction("QueryAccount", id)
        if err != nil {
            w.WriteHeader(http.StatusInternalServerError)
            fmt.Fprintf(w, "QueryAccount failed: %v", err)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        w.Write(result)
    }).Methods("GET")

    r.HandleFunc("/accounts/{id}/balance", func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        id := vars["id"]
        newBalance := r.FormValue("balance")
        _, err := contract.SubmitTransaction("UpdateAccountBalance", id, newBalance)
        if err != nil {
            w.WriteHeader(http.StatusInternalServerError)
            fmt.Fprintf(w, "UpdateAccountBalance failed: %v", err)
            return
        }
        fmt.Fprintf(w, "Balance updated for %s", id)
    }).Methods("PUT")

    r.HandleFunc("/accounts/{id}/history", func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        id := vars["id"]
        result, err := contract.EvaluateTransaction("GetAccountHistory", id)
        if err != nil {
            w.WriteHeader(http.StatusInternalServerError)
            fmt.Fprintf(w, "GetAccountHistory failed: %v", err)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        w.Write(result)
    }).Methods("GET")

    srv := &http.Server{
        Handler:      r,
        Addr:         ":8080",
        WriteTimeout: 15 * time.Second,
        ReadTimeout:  15 * time.Second,
    }

    fmt.Println("REST API listening on :8080")
    log.Fatal(srv.ListenAndServe())
}
