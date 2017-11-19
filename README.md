# hayden

[![GoDoc](https://godoc.org/github.com/icco/hayden?status.svg)](https://godoc.org/github.com/icco/hayden)

An app for making sure things are archived

## Internet Archive Save Example

```
$ curl -svL https://web.archive.org/save/https://natwelch.com/ > /dev/null
*   Trying 207.241.225.186...
* TCP_NODELAY set
* Connected to web.archive.org (207.241.225.186) port 443 (#0)
* ALPN, offering h2
* ALPN, offering http/1.1
* Cipher selection: ALL:!EXPORT:!EXPORT40:!EXPORT56:!aNULL:!LOW:!RC4:@STRENGTH
* successfully set certificate verify locations:
*   CAfile: /etc/ssl/cert.pem
  CApath: none
* TLSv1.2 (OUT), TLS handshake, Client hello (1):
* TLSv1.2 (IN), TLS handshake, Server hello (2):
* TLSv1.2 (IN), TLS handshake, Certificate (11):
* TLSv1.2 (IN), TLS handshake, Server key exchange (12):
* TLSv1.2 (IN), TLS handshake, Server finished (14):
* TLSv1.2 (OUT), TLS handshake, Client key exchange (16):
* TLSv1.2 (OUT), TLS change cipher, Client hello (1):
* TLSv1.2 (OUT), TLS handshake, Finished (20):
* TLSv1.2 (IN), TLS change cipher, Client hello (1):
* TLSv1.2 (IN), TLS handshake, Finished (20):
* SSL connection using TLSv1.2 / ECDHE-RSA-AES128-GCM-SHA256
* ALPN, server accepted to use http/1.1
* Server certificate:
*  subject: OU=Domain Control Validated; CN=*.archive.org
*  start date: Dec 19 20:59:01 2016 GMT
*  expire date: Feb 21 22:56:08 2020 GMT
*  subjectAltName: host "web.archive.org" matched cert's "*.archive.org"
*  issuer: C=US; ST=Arizona; L=Scottsdale; O=GoDaddy.com, Inc.; OU=http://certs.godaddy.com/repository/; CN=Go Daddy Secure Certificate Authority - G2
*  SSL certificate verify ok.
> GET /save/https://natwelch.com/ HTTP/1.1
> Host: web.archive.org
> User-Agent: curl/7.54.0
> Accept: */*
>
< HTTP/1.1 200 OK
< Server: Tengine/2.1.0
< Date: Sat, 18 Nov 2017 11:43:52 GMT
< Content-Type: text/html;charset=utf-8
< Content-Length: 9455
< Connection: keep-alive
< Content-Location: /web/20171118114351/https://natwelch.com/
< Set-Cookie: JSESSIONID=E5A41DE4284A3C0A276D921FBE227A8D; Path=/; HttpOnly
< X-Archive-Orig-X-XSS-Protection: 1; mode=block
< X-Archive-Orig-Strict-Transport-Security: max-age=31536000;
< X-Archive-Guessed-Charset: utf-8
< X-Archive-Orig-X-Frame-Options: SAMEORIGIN
< X-Archive-Orig-Server: nginx
< X-Archive-Orig-Connection: close
< X-Archive-Orig-Last-Modified: Wed, 27 Sep 2017 01:46:24 GMT
< X-Archive-Orig-Alt-Svc: clear
< X-Archive-Orig-Date: Sat, 18 Nov 2017 11:43:52 GMT
< X-Archive-Orig-Content-Length: 6336
< X-Archive-Orig-Accept-Ranges: bytes
< X-Archive-Orig-ETag: "59cb02f0-18c0"
< X-Archive-Orig-Content-Type: text/html; charset=utf-8
< X-Archive-Orig-Via: 1.1 google
< X-Archive-Orig-Cache-Control: public
< X-Archive-Orig-Expires: Sat, 18 Nov 2017 12:43:52 GMT
< X-Archive-Orig-X-Content-Type-Options: nosniff
< X-App-Server: wwwb-app8
< X-ts: ----
< X-Archive-Playback: 1
< X-location: save
< X-Page-Cache: MISS
<
```
