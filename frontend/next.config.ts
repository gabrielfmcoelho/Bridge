import type { NextConfig } from "next";

const apiUrl = process.env.API_URL || "http://localhost:8080";

const nextConfig: NextConfig = {
  output: "standalone",
  images: {
    unoptimized: true,
  },
  // The bulk DNS cert scan is synchronous and can outlast Next's 30s default;
  // match the 10-minute client timeout in dnsAPI.scanCerts.
  experimental: {
    proxyTimeout: 10 * 60_000,
  },
  allowedDevOrigins: ['127.0.0.1', 'localhost', '192.168.15.90', '100.78.26.92', '100.74.185.3'],
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${apiUrl}/api/:path*`,
      },
    ];
  },
  async redirects() {
    return [
      // Phase 4 Task 4.2: legacy bookmarks landing on /service-credentials
      // are forwarded to the unified vault page with the scope chip pre-
      // selected. permanent: true so browsers cache the 308 and search
      // crawlers update their index.
      {
        source: "/service-credentials",
        destination: "/secrets?scope=service",
        permanent: true,
      },
    ];
  },
};

export default nextConfig;
