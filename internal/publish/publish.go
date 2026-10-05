package publish

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// ZeroKnowledgePackage contains encrypted payload and client decryption fragment.
type ZeroKnowledgePackage struct {
	SubscriptionID string `json:"subscription_id"`
	SecretKeyHex   string `json:"secret_key_hex"`
	CiphertextB64  string `json:"ciphertext_b64"`
	ZeroKnowledgeURL string `json:"zero_knowledge_url"`
}

// EncryptSubscription encrypts subscription plaintext using AES-256-GCM.
func EncryptSubscription(plaintext []byte, workerEndpoint string) (*ZeroKnowledgePackage, error) {
	// 1. Generate 256-bit key
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// 2. Generate random 12-byte Nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	// 3. Encrypt payload
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	cipherB64 := base64.StdEncoding.EncodeToString(ciphertext)

	// 4. Generate random Subscription ID
	subIDBytes := make([]byte, 8)
	_, _ = rand.Read(subIDBytes)
	subID := hex.EncodeToString(subIDBytes)

	keyHex := hex.EncodeToString(key[:])

	if workerEndpoint == "" {
		workerEndpoint = "https://sub.your-domain.workers.dev"
	}

	// URL Fragment (#...) is NEVER transmitted to the server in HTTP headers (RFC 3986)
	// Ensuring Zero-Knowledge: Worker server only sees /sub?id=xyz, never the AES key!
	zkURL := fmt.Sprintf("%s/sub?id=%s#%s", workerEndpoint, subID, keyHex)

	return &ZeroKnowledgePackage{
		SubscriptionID:   subID,
		SecretKeyHex:     keyHex,
		CiphertextB64:    cipherB64,
		ZeroKnowledgeURL: zkURL,
	}, nil
}

// UploadToWorker pushes the encrypted payload to the Cloudflare Worker.
func UploadToWorker(ctx context.Context, workerURL, secret string, pkg *ZeroKnowledgePackage) error {
	endpoint := strings.TrimRight(workerURL, "/") + "/api/publish"
	payload := map[string]string{
		"id":         pkg.SubscriptionID,
		"ciphertext": pkg.CiphertextB64,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("worker returned status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// GenerateCloudflareWorkerScript returns a production-ready Cloudflare Worker script
// that stores ciphertext and serves zero-knowledge client decryption.
func GenerateCloudflareWorkerScript() string {
	return `// vpnctl Zero-Knowledge Subscription Cloudflare Worker
export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    const id = url.searchParams.get("id");

    if (request.method === "POST" && url.pathname === "/api/publish") {
      const auth = request.headers.get("Authorization");
      if (env.API_SECRET && auth !== "Bearer " + env.API_SECRET) {
        return new Response("Unauthorized", { status: 401 });
      }
      const data = await request.json();
      await env.SUB_KV.put(data.id, data.ciphertext);
      return new Response(JSON.stringify({ success: true, id: data.id }), {
        headers: { "Content-Type": "application/json" }
      });
    }

    if (url.pathname.startsWith("/sub")) {
      if (!id) return new Response("Missing subscription id (?id=...)", { status: 400 });
      const ciphertext = await env.SUB_KV.get(id);
      if (!ciphertext) return new Response("Subscription not found or expired", { status: 404 });

      const accept = request.headers.get("accept") || "";
      if (accept.includes("text/html")) {
        return new Response(renderWebCryptoDecryptorHTML(ciphertext), {
          headers: { "Content-Type": "text/html; charset=utf-8" }
        });
      }

      return new Response(ciphertext, {
        headers: {
          "Content-Type": "text/plain; charset=utf-8",
          "Cache-Control": "no-store, no-cache",
          "Access-Control-Allow-Origin": "*"
        }
      });
    }

    return new Response("vpnctl Zero-Knowledge Gateway Active", { status: 200 });
  }
};

function renderWebCryptoDecryptorHTML(cipherB64) {
  return ` + "`" + `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>vpnctl Zero-Knowledge Subscription</title>
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    :root { --bg: #0d1117; --card: #161b22; --accent: #58a6ff; --text: #c9d1d9; --success: #3fb950; }
    body { background: var(--bg); color: var(--text); font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; display: flex; justify-content: center; align-items: center; min-height: 100vh; margin: 0; padding: 20px; box-sizing: border-box; }
    .card { background: var(--card); border: 1px solid #30363d; border-radius: 12px; max-width: 680px; width: 100%; padding: 32px; box-shadow: 0 16px 32px rgba(0,0,0,0.5); }
    h2 { margin-top: 0; color: #fff; display: flex; align-items: center; gap: 8px; font-size: 20px; }
    .badge { display: inline-flex; align-items: center; padding: 6px 12px; background: rgba(63, 185, 80, 0.15); color: var(--success); border-radius: 20px; font-size: 13px; font-weight: 600; margin-bottom: 20px; }
    pre { background: #090d13; border: 1px solid #30363d; padding: 16px; border-radius: 8px; font-size: 12px; overflow-x: auto; max-height: 340px; color: #79c0ff; }
    button { background: var(--accent); color: #fff; border: none; padding: 10px 18px; border-radius: 6px; font-weight: 600; cursor: pointer; transition: 0.2s; font-size: 14px; }
    button:hover { background: #388bfd; }
    .actions { display: flex; gap: 12px; margin-top: 20px; }
  </style>
</head>
<body>
  <div class="card">
    <h2>🛡️ Zero-Knowledge Subscription</h2>
    <div class="badge">🔒 End-to-End Encrypted (AES-256-GCM)</div>
    <p style="font-size: 14px; line-height: 1.5; color: #8b949e;">
      The server stores only high-entropy ciphertext. Your configuration is decrypted strictly in this local browser session using the secret key from the URL hash.
    </p>
    <div id="status" style="font-weight: 600; margin-bottom: 12px;">Decrypting configuration...</div>
    <pre id="output">Loading...</pre>
    <div class="actions">
      <button onclick="copyConfig()">📋 Copy Decrypted Profile</button>
      <button onclick="downloadConfig()" style="background: #238636;">💾 Download .yaml</button>
    </div>
  </div>
  <script>
    const cipherB64 = "` + "`" + ` + "` + "${cipherB64}" + `" + ` + "`" + `;
    async function decrypt() {
      try {
        const keyHex = window.location.hash.replace('#', '').trim();
        if (!keyHex || keyHex.length !== 64) {
          document.getElementById('status').innerHTML = '<span style="color:#f85149">⚠️ Missing or invalid key in URL hash fragment!</span>';
          document.getElementById('output').textContent = 'Cannot decrypt without the #AES_KEY anchor in URL.';
          return;
        }
        const keyBytes = new Uint8Array(keyHex.match(/.{1,2}/g).map(byte => parseInt(byte, 16)));
        const rawCipher = Uint8Array.from(atob(cipherB64), c => c.charCodeAt(0));
        const nonce = rawCipher.slice(0, 12);
        const data = rawCipher.slice(12);

        const cryptoKey = await crypto.subtle.importKey('raw', keyBytes, { name: 'AES-GCM' }, false, ['decrypt']);
        const decrypted = await crypto.subtle.decrypt({ name: 'AES-GCM', iv: nonce }, cryptoKey, data);
        const text = new TextDecoder().decode(decrypted);

        document.getElementById('status').innerHTML = '<span style="color:#3fb950">✅ Decrypted Successfully (100% Client-Side)</span>';
        document.getElementById('output').textContent = text;
        window.decryptedText = text;
      } catch (err) {
        document.getElementById('status').innerHTML = '<span style="color:#f85149">❌ Decryption Failed: ' + err.message + '</span>';
      }
    }
    function copyConfig() {
      if (window.decryptedText) {
        navigator.clipboard.writeText(window.decryptedText);
        alert('Decrypted profile copied to clipboard!');
      }
    }
    function downloadConfig() {
      if (window.decryptedText) {
        const blob = new Blob([window.decryptedText], { type: 'text/yaml' });
        const a = document.createElement('a');
        a.href = URL.createObjectURL(blob);
        a.download = 'clash-profile.yaml';
        a.click();
      }
    }
    decrypt();
  </script>
</body>
</html>` + "`" + `;
}

// SaveWorkerScript saves the Cloudflare Worker script locally.
func SaveWorkerScript(destPath string) error {
	_ = os.MkdirAll(filepath.Dir(destPath), 0755)
	return os.WriteFile(destPath, []byte(GenerateCloudflareWorkerScript()), 0644)
}

