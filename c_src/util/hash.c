#include "hash.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <openssl/evp.h>

static char *hex_encode(const unsigned char *digest, unsigned int digest_len) {
    /* "sha256:" + 64 hex chars + NUL */
    char *out = malloc(7 + (digest_len * 2) + 1);
    if (!out) return NULL;
    memcpy(out, "sha256:", 7);
    for (unsigned int i = 0; i < digest_len; i++)
        snprintf(out + 7 + i * 2, 3, "%02x", digest[i]);
    return out;
}

char *hash_bytes(const unsigned char *data, size_t len) {
    EVP_MD_CTX *ctx = EVP_MD_CTX_new();
    unsigned char digest[EVP_MAX_MD_SIZE];
    unsigned int digest_len = 0;
    char *out = NULL;

    if (!ctx) return NULL;
    if (EVP_DigestInit_ex(ctx, EVP_sha256(), NULL) != 1) goto done;
    if (EVP_DigestUpdate(ctx, data, len) != 1) goto done;
    if (EVP_DigestFinal_ex(ctx, digest, &digest_len) != 1) goto done;

    out = hex_encode(digest, digest_len);

done:
    EVP_MD_CTX_free(ctx);
    return out;
}

char *hash_file(const char *path) {
    FILE *f = fopen(path, "rb");
    if (!f) { perror(path); return NULL; }

    EVP_MD_CTX *ctx = EVP_MD_CTX_new();
    if (!ctx) {
        fclose(f);
        return NULL;
    }

    if (EVP_DigestInit_ex(ctx, EVP_sha256(), NULL) != 1) {
        EVP_MD_CTX_free(ctx);
        fclose(f);
        return NULL;
    }

    unsigned char buf[65536];
    size_t n;
    while ((n = fread(buf, 1, sizeof(buf), f)) > 0) {
        if (EVP_DigestUpdate(ctx, buf, n) != 1) {
            EVP_MD_CTX_free(ctx);
            fclose(f);
            return NULL;
        }
    }

    if (ferror(f)) {
        EVP_MD_CTX_free(ctx);
        fclose(f);
        return NULL;
    }

    fclose(f);

    unsigned char digest[EVP_MAX_MD_SIZE];
    unsigned int digest_len = 0;
    if (EVP_DigestFinal_ex(ctx, digest, &digest_len) != 1) {
        EVP_MD_CTX_free(ctx);
        return NULL;
    }

    EVP_MD_CTX_free(ctx);
    return hex_encode(digest, digest_len);
}
