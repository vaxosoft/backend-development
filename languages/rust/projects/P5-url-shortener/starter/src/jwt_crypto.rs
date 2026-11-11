//! Provided helper: the HS256 crypto backend for `jsonwebtoken`. Nothing to do here.
//!
//! `jsonwebtoken` 11 ships without a crypto backend unless you enable its
//! `rust_crypto` or `aws_lc_rs` feature. This workspace pins the crate with its
//! default features, so we install a tiny HS256 provider built on `hmac` + `sha2`.
//! In your own project, enable one of the two features instead and delete this file.

use std::sync::Once;

use hmac::{Hmac, KeyInit, Mac};
use jsonwebtoken::crypto::{CryptoProvider, JwtSigner, JwtVerifier, KeyUtils};
use jsonwebtoken::errors::{Error, ErrorKind, Result};
use jsonwebtoken::signature::{Error as SignatureError, Signer, Verifier};
use jsonwebtoken::{Algorithm, DecodingKey, EncodingKey};
use sha2::Sha256;

type HmacSha256 = Hmac<Sha256>;

struct Hs256Signer(HmacSha256);
struct Hs256Verifier(HmacSha256);

impl Signer<Vec<u8>> for Hs256Signer {
    fn try_sign(&self, msg: &[u8]) -> std::result::Result<Vec<u8>, SignatureError> {
        let mut mac = self.0.clone();
        mac.update(msg);
        Ok(mac.finalize().into_bytes().to_vec())
    }
}

impl JwtSigner for Hs256Signer {
    fn algorithm(&self) -> Algorithm {
        Algorithm::HS256
    }
}

impl Verifier<Vec<u8>> for Hs256Verifier {
    fn verify(&self, msg: &[u8], signature: &Vec<u8>) -> std::result::Result<(), SignatureError> {
        let mut mac = self.0.clone();
        mac.update(msg);
        // verify_slice compares in constant time
        mac.verify_slice(signature)
            .map_err(SignatureError::from_source)
    }
}

impl JwtVerifier for Hs256Verifier {
    fn algorithm(&self) -> Algorithm {
        Algorithm::HS256
    }
}

fn signer(alg: &Algorithm, key: &EncodingKey) -> Result<Box<dyn JwtSigner>> {
    if *alg != Algorithm::HS256 {
        return Err(Error::from(ErrorKind::InvalidAlgorithm));
    }
    let mac = HmacSha256::new_from_slice(key.as_bytes())
        .map_err(|_| Error::from(ErrorKind::InvalidKeyFormat))?;
    Ok(Box::new(Hs256Signer(mac)))
}

fn verifier(alg: &Algorithm, key: &DecodingKey) -> Result<Box<dyn JwtVerifier>> {
    if *alg != Algorithm::HS256 {
        return Err(Error::from(ErrorKind::InvalidAlgorithm));
    }
    let mac = HmacSha256::new_from_slice(key.try_get_as_bytes()?)
        .map_err(|_| Error::from(ErrorKind::InvalidKeyFormat))?;
    Ok(Box::new(Hs256Verifier(mac)))
}

static PROVIDER: CryptoProvider = CryptoProvider {
    signer_factory: signer,
    verifier_factory: verifier,
    key_utils: KeyUtils::new_unimplemented(),
};

/// Installs the HS256 provider for this process. Safe to call many times, and it
/// must run before the first `encode`/`decode`.
pub fn install_jwt_crypto() {
    static INSTALL: Once = Once::new();
    INSTALL.call_once(|| {
        // Err only means a provider was installed already, which is fine.
        let _ = PROVIDER.install_default();
    });
}
