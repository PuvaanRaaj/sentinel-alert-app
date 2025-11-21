#!/bin/bash
body='{"title":"CPU High","message":"95% usage","level":"warning","level":"warning","chat_id":"chat_1_1763699534780299773"}'
secret='supersecret'
sig=$(printf "%s" "$body" | openssl dgst -sha256 -hmac "$secret" -binary | xxd -p -c 256)

curl -X POST "http://localhost:8080/bot/843d03a645deb835c74206afea0c2f0140cb18c7c61529b52bab8a2a614708c3/sendMessage" \
  -H "Content-Type: application/json" \
  -H "X-Sentinel-Signature: $sig" \
  -d "$body"