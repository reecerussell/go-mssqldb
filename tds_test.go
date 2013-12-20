package mssql

import (
    "database/sql"
    "os"
    "testing"
    "bytes"
    "fmt"
    "encoding/hex"
)

type MockTransport struct {
    bytes.Buffer
}


func (t *MockTransport) Close() error {
    return nil
}


func TestSendLogin(t *testing.T) {
    buf := NewTdsBuffer(1024, new(MockTransport))
    login := Login{
        TDSVersion: TDS73,
        PacketSize: 0x1000,
        ClientProgVer: 0x01060100,
        ClientPID: 100,
        ClientTimeZone: -4 * 60,
        ClientID: [6]byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab},
        OptionFlags1: 0xe0,
        OptionFlags3: 8,
        HostName: "subdev1",
        UserName: "test",
        Password: "testpwd",
        AppName: "appname",
        ServerName: "servername",
        CtlIntName: "library",
        Language: "en",
        Database: "database",
        ClientLCID: 0x204,
        AtchDBFile: "filepath",
    }
    err := SendLogin(buf, login)
    if err != nil {
        t.Error("SendLogin should succeed")
    }
    ref := []byte{
        16, 1, 0, 222, 0, 0, 0, 0, 198+16, 0, 0, 0, 3, 0, 10, 115, 0, 16, 0, 0, 0, 1,
        6, 1, 100, 0, 0, 0, 0, 0, 0, 0, 224, 0, 0, 8, 16, 255, 255, 255, 4, 2, 0,
        0, 94, 0, 7, 0, 108, 0, 4, 0, 116, 0, 7, 0, 130, 0, 7, 0, 144, 0, 10, 0, 0,
        0, 0, 0, 164, 0, 7, 0, 178, 0, 2, 0, 182, 0, 8, 0, 18, 52, 86, 120, 144, 171,
        198, 0, 0, 0, 198, 0, 8, 0, 214, 0, 0, 0, 0, 0, 0, 0, 115, 0, 117, 0, 98,
        0, 100, 0, 101, 0, 118, 0, 49, 0, 116, 0, 101, 0, 115, 0, 116, 0, 226, 165,
        243, 165, 146, 165, 226, 165, 162, 165, 210, 165, 227, 165, 97, 0, 112,
        0, 112, 0, 110, 0, 97, 0, 109, 0, 101, 0, 115, 0, 101, 0, 114, 0, 118, 0,
        101, 0, 114, 0, 110, 0, 97, 0, 109, 0, 101, 0, 108, 0, 105, 0, 98, 0, 114,
        0, 97, 0, 114, 0, 121, 0, 101, 0, 110, 0, 100, 0, 97, 0, 116, 0, 97, 0, 98,
        0, 97, 0, 115, 0, 101, 0, 102, 0, 105, 0, 108, 0, 101, 0, 112, 0, 97, 0,
        116, 0, 104, 0}
    out := buf.buf[:buf.pos]
    if !bytes.Equal(ref, out) {
        t.Error("input output don't match")
        fmt.Print(hex.Dump(ref))
        fmt.Print(hex.Dump(out))
    }
}


func TestSendSqlBatch(t *testing.T) {
    addr := os.Getenv("HOST")
    instance := os.Getenv("INSTANCE")

    conn, err := Connect(map[string]string {
        "server": fmt.Sprintf("%s\\%s", addr, instance),
        "user id": os.Getenv("SQLUSER"),
        "password": os.Getenv("SQLPASSWORD"),
        })
    if err != nil {
        t.Error("Open connection failed:", err.Error())
    }
    defer conn.buf.transport.Close()

    headers := []headerStruct{
        {hdrtype: dataStmHdrTransDescr,
         data: transDescrHdr{0, 1}.pack()},
    }
    err = sendSqlBatch72(conn.buf, "select 1", headers)
    if err != nil {
        t.Error("Sending sql batch failed", err.Error())
    }

    err = processResponse(conn)
    if err != nil {
        t.Error("Processing reponse failed", err.Error())
    }

    switch value := conn.lastRow[0].(type) {
    case int32:
        if value != 1 {
            t.Error("Invalid value returned, should be 1", value)
        }
    }
}


func makeConnStr() string {
    addr := os.Getenv("HOST")
    instance := os.Getenv("INSTANCE")
    user := os.Getenv("SQLUSER")
    password := os.Getenv("SQLPASSWORD")
    return fmt.Sprintf(
        "Server=%s\\%s;User Id=%s;Password=%s",
        addr, instance, user, password)
}


func open(t *testing.T) *sql.DB {
    conn, err := sql.Open("go-mssql", makeConnStr())
    if err != nil {
        t.Error("Open connection failed:", err.Error())
    }
    return conn
}


func TestConnect(t *testing.T) {
    conn, err := sql.Open("go-mssql", makeConnStr())
    if err != nil {
        t.Error("Open connection failed:", err.Error())
    }
    defer conn.Close()
}


func TestBadConnect(t *testing.T) {
    badDsns := []string{
        //"Server=badhost",
        fmt.Sprintf("Server=%s\\%s;User ID=baduser;Password=badpwd",
                    os.Getenv("HOST"), os.Getenv("INSTANCE")),
    }
    for _, badDsn := range badDsns {
        conn, err := sql.Open("go-mssql", badDsn)
        if err != nil {
            t.Error("Open connection failed:", err.Error())
        }
        defer conn.Close()
        err = conn.Ping()
        if err == nil {
            t.Error("Ping should fail for connection: ", badDsn)
        }
    }
}


func TestQuery(t *testing.T) {
    conn := open(t)
    defer conn.Close()

    stmt, err := conn.Prepare("select 1")
    if err != nil {
        t.Error("Prepare failed:", err.Error())
    }
    defer stmt.Close()

    rows, err := stmt.Query()
    if err != nil {
        t.Error("Query failed:", err.Error())
    }
    defer rows.Close()

    for rows.Next() {
        var val int
        err := rows.Scan(&val)
        if err != nil {
            t.Error("Scan failed:", err.Error())
        }
        if val != 1 {
            t.Error("query should return 1")
        }
    }
}


func TestPing(t *testing.T) {
    conn := open(t)
    defer conn.Close()
    conn.Ping()
}
