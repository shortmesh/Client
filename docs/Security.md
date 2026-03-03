## Security

### API Request
#### Authentication: HMAC Request Signing

This project uses HMAC-SHA256 Request Signing instead of static bearer tokens. This ensures message integrity, origin authenticity, and protection against replay attacks.
How it Works

Every request must include a set of custom HTTP headers. These headers prove the sender possesses the Secret Key without ever transmitting the key over the network.
Required Headers
Header	Description	Example
```
X-ShortMesh-ID	-> Your MAS client ID.	platform-alpha-01

X-ShortMesh-Timestamp	ISO8601 UTC timestamp.	2026-03-03T14:23:36Z

X-ShortMesh-Nonce	A random unique string (min 16 chars).	7f3a1b2c5d...

X-ShortMesh-Signature	The computed HMAC-SHA256 hex string.	e3b0c442...

Signature Calculation...
```

To generate the X-ShortMesh-Signature, concatenate the following fields in order into a single Canonical String (no delimiters):

```
# Calculate Signature
string_to_sign = ID + Method + Path + Timestamp + Nonce + Body

Secret = `MAS client secret`
signature = HMAC_SHA256(key=Secret, message=string_to_sign).to_hex()
```

#### Security Constraints
- Clock Skew: Requests with a timestamp differing by more than 30 seconds from the server time will be rejected.
- Nonce Uniqueness: Each nonce can only be used once per 60-second window.
- Body Integrity: Any modification to the request body after signing will result in a signature mismatch.
