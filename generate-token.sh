key=$(cat "tokens.key")
echo -n $1 | openssl dgst -sha256 -mac HMAC -macopt hexkey:$key -binary | openssl enc -base64 -A