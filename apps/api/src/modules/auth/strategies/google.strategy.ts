import { config } from "../../../config";

interface GoogleTokens {
  access_token: string;
  id_token: string;
}

interface GoogleUser {
  sub: string; // Google ID
  email: string;
  name: string;
  picture: string;
  email_verified: boolean;
}

/**
 * Google OAuth Strategy — tạo auth URL, exchange code, lấy user info.
 * Dùng OpenID Connect flow (response_type=code).
 */
export class GoogleStrategy {
  /** Tạo URL redirect đến Google OAuth consent screen. */
  getAuthUrl(): string {
    const params = new URLSearchParams({
      client_id: config.GOOGLE_CLIENT_ID,
      redirect_uri: config.GOOGLE_REDIRECT_URI,
      response_type: "code",
      scope: "openid email profile",
      access_type: "offline",
      prompt: "consent",
    });
    return `https://accounts.google.com/o/oauth2/v2/auth?${params}`;
  }

  /** Exchange authorization code → tokens. */
  async exchangeCode(code: string): Promise<GoogleTokens> {
    const res = await fetch("https://oauth2.googleapis.com/token", {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams({
        code,
        client_id: config.GOOGLE_CLIENT_ID,
        client_secret: config.GOOGLE_CLIENT_SECRET,
        redirect_uri: config.GOOGLE_REDIRECT_URI,
        grant_type: "authorization_code",
      }),
    });

    if (!res.ok) {
      throw new Error(`Google token exchange failed: ${await res.text()}`);
    }

    return res.json() as Promise<GoogleTokens>;
  }

  /** Lấy thông tin user từ Google. */
  async getUserInfo(accessToken: string): Promise<GoogleUser> {
    const res = await fetch("https://www.googleapis.com/oauth2/v3/userinfo", {
      headers: { Authorization: `Bearer ${accessToken}` },
    });

    if (!res.ok) {
      throw new Error(`Google userinfo failed: ${await res.text()}`);
    }

    return res.json() as Promise<GoogleUser>;
  }
}
