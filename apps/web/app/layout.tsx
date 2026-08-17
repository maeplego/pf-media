export const dynamic = "force-dynamic";

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="ja">
      <body style={{ fontFamily: "system-ui, sans-serif", margin: "2rem", maxWidth: 720 }}>
        <h1>Media Drive</h1>
        {children}
      </body>
    </html>
  );
}
