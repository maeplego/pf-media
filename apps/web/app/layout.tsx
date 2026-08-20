import "./globals.css";

export const dynamic = "force-dynamic";

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="ja">
      <body>
        <div className="site-shell">
          <header className="site-header">
            <div className="site-brand">
              <a href="/" className="brand-link">
                <strong>Media Drive</strong>
              </a>
              <span className="muted">P03 学習用ファイルドライブ</span>
            </div>
            <nav className="site-nav">
              <a href="/">ドライブ</a>
            </nav>
          </header>
          <main className="site-main">{children}</main>
        </div>
      </body>
    </html>
  );
}
