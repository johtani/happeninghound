if (Get-BwsToken) {
    bws run -- .\happeninghound.exe
    Clear-BwsToken
}