"use client";

import Image from "next/image";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import {
  ArrowUpRight,
  Check,
  LogOut,
  Mail,
  RefreshCw,
  ShieldCheck,
} from "lucide-react";

type UserProfile = {
  id: string;
  email: string;
  emailVerified: boolean;
  name: string;
  pictureUrl: string;
};

export default function Home() {
  const [user, setUser] = useState<UserProfile | null>(null);
  const [apiURL, setApiURL] = useState("http://localhost:8080");
  const [isLoading, setIsLoading] = useState(true);
  const [isSigningOut, setIsSigningOut] = useState(false);
  const [error, setError] = useState("");

  const loadUser = useCallback(async () => {
    setIsLoading(true);
    setError("");

    try {
      const configResponse = await fetch("/api/config", { cache: "no-store" });
      if (!configResponse.ok) throw new Error("Could not load account service settings.");
      const config = (await configResponse.json()) as { backendURL: string };
      const backendURL = config.backendURL.replace(/\/$/, "");
      setApiURL(backendURL);

      const response = await fetch(`${backendURL}/api/me`, {
        credentials: "include",
        cache: "no-store",
      });

      if (response.status === 401) {
        setUser(null);
        return;
      }
      if (!response.ok) {
        throw new Error(response.status === 503 ? "Google sign-in is not configured yet." : "Could not load your account.");
      }

      setUser((await response.json()) as UserProfile);
    } catch (fetchError) {
      setUser(null);
      setError(fetchError instanceof Error ? fetchError.message : "Could not connect to the account service.");
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void Promise.resolve().then(loadUser);
  }, [loadUser]);

  async function signOut() {
    setIsSigningOut(true);
    setError("");
    try {
      const response = await fetch(`${apiURL}/auth/logout`, {
        method: "POST",
        credentials: "include",
      });
      if (!response.ok) throw new Error("Could not sign out. Please try again.");
      setUser(null);
    } catch (fetchError) {
      setError(fetchError instanceof Error ? fetchError.message : "Could not sign out. Please try again.");
    } finally {
      setIsSigningOut(false);
    }
  }

  return (
    <main className="account-shell">
      <div className="ambient ambient-one" aria-hidden="true" />
      <div className="ambient ambient-two" aria-hidden="true" />

      <header className="topbar">
        <Link className="brand" href="/" aria-label="Deploy Test home">
          <span className="brand-mark" aria-hidden="true">
            d<span>.</span>
          </span>
          <span className="brand-name">deploy test</span>
        </Link>
        <span className="help-link"><ShieldCheck size={16} strokeWidth={1.8} /> Secure sign-in</span>
      </header>

      <section className="account-content" aria-live="polite">
        {isLoading ? (
          <LoadingCard />
        ) : user ? (
          <ProfileCard user={user} isSigningOut={isSigningOut} error={error} onSignOut={signOut} />
        ) : (
          <SignInCard apiURL={apiURL} error={error} onRetry={loadUser} />
        )}
      </section>

      <footer className="page-footer">
        <span>Deploy Test account</span>
        <span className="footer-dot" aria-hidden="true">·</span>
        <span>Secure account access</span>
      </footer>
    </main>
  );
}

function LoadingCard() {
  return (
    <div className="loading-card" role="status">
      <span className="loader-ring" aria-hidden="true" />
      <p>Checking your account</p>
    </div>
  );
}

function SignInCard({ apiURL, error, onRetry }: { apiURL: string; error: string; onRetry: () => void }) {
  return (
    <div className="auth-card sign-in-card">
      <div className="eyebrow"><span className="eyebrow-line" /> YOUR SPACE, READY</div>
      <h1>Good work starts<br />with a clear view.</h1>
      <p className="intro-copy">Sign in to your workspace to pick up where you left off.</p>

      {error && (
        <div className="notice notice-error" role="alert">
          <span>{error}</span>
          <button className="text-button" onClick={onRetry} type="button">Try again</button>
        </div>
      )}

      <a className="google-button" href={`${apiURL}/auth/google`}>
        <GoogleMark />
        <span>Continue with Google</span>
        <ArrowUpRight className="button-arrow" size={17} strokeWidth={1.8} />
      </a>

      <div className="security-note">
        <ShieldCheck size={16} strokeWidth={1.8} />
        <span>Your account is protected with secure sign-in.</span>
      </div>

      <div className="card-rule" />
      <p className="terms-copy">Your Google profile details are used to personalize your account.</p>
    </div>
  );
}

function ProfileCard({
  user,
  isSigningOut,
  error,
  onSignOut,
}: {
  user: UserProfile;
  isSigningOut: boolean;
  error: string;
  onSignOut: () => void;
}) {
  const initials = user.name.trim().split(/\s+/).slice(0, 2).map((part) => part[0]).join("").toUpperCase();

  return (
    <div className="auth-card profile-card">
      <div className="profile-heading">
        <div className="eyebrow"><span className="eyebrow-line" /> YOUR ACCOUNT</div>
        <span className="signed-in-tag"><span /> SIGNED IN</span>
      </div>

      <div className="profile-identity">
        <div className="avatar-wrap">
          {user.pictureUrl ? (
            <Image className="profile-avatar" src={user.pictureUrl} alt="" width={76} height={76} unoptimized />
          ) : (
            <span className="profile-avatar avatar-fallback">{initials || "U"}</span>
          )}
          <span className="avatar-check"><Check size={13} strokeWidth={2.6} /></span>
        </div>
        <div className="identity-copy">
          <p className="welcome-label">Welcome back</p>
          <h1>{user.name || "Your account"}</h1>
          <p className="identity-email">{user.email}</p>
        </div>
      </div>

      {error && <div className="notice notice-error" role="alert">{error}</div>}

      <div className="details-panel">
        <div className="details-heading">
          <span>ACCOUNT DETAILS</span>
          <span className={`verified-label${user.emailVerified ? "" : " unverified-label"}`}>
            <ShieldCheck size={14} /> {user.emailVerified ? "VERIFIED" : "NOT VERIFIED"}
          </span>
        </div>
        <div className="detail-row">
          <span className="detail-icon"><Mail size={17} strokeWidth={1.8} /></span>
          <span className="detail-label">Email address</span>
          <span className="detail-value">{user.email}</span>
        </div>
        <div className="detail-row account-id-row">
          <span className="detail-icon id-icon">#</span>
          <span className="detail-label">Account ID</span>
          <span className="detail-value account-id">{user.id}</span>
        </div>
      </div>

      <button className="sign-out-button" onClick={onSignOut} disabled={isSigningOut} type="button">
        {isSigningOut ? <RefreshCw className="spin" size={16} /> : <LogOut size={16} strokeWidth={1.8} />}
        <span>{isSigningOut ? "Signing out…" : "Sign out"}</span>
      </button>

      <div className="security-note profile-security-note">
        <ShieldCheck size={16} strokeWidth={1.8} />
        <span>Your profile is synced securely with your Google account.</span>
      </div>
    </div>
  );
}

function GoogleMark() {
  return (
    <svg aria-hidden="true" className="google-mark" viewBox="0 0 48 48">
      <path fill="#4285F4" d="M43.6 24.5c0-1.4-.1-2.8-.4-4.1H24v7.8h11a9.4 9.4 0 0 1-4.1 6.2v5.1h6.6c3.9-3.6 6.1-8.8 6.1-15Z" />
      <path fill="#34A853" d="M24 44c5.5 0 10.1-1.8 13.5-4.9l-6.6-5.1c-1.8 1.2-4.1 2-6.9 2-5.3 0-9.8-3.6-11.4-8.4H5.8v5.3A20 20 0 0 0 24 44Z" />
      <path fill="#FBBC05" d="M12.6 27.6a12 12 0 0 1 0-7.2v-5.3H5.8a20 20 0 0 0 0 17.8l6.8-5.3Z" />
      <path fill="#EA4335" d="M24 12c3 0 5.7 1 7.8 3l5.8-5.8A19.4 19.4 0 0 0 24 4 20 20 0 0 0 5.8 15.1l6.8 5.3C14.2 15.6 18.7 12 24 12Z" />
    </svg>
  );
}
