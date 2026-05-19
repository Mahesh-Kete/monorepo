/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  output: 'standalone',
  async rewrites() {
    const backend = process.env.BACKEND_URL || 'http://backend:8080';
    return [
      { source: '/api/:path*', destination: `${backend}/api/:path*` },
      { source: '/healthz', destination: `${backend}/healthz` },
    ];
  },
};

export default nextConfig;
