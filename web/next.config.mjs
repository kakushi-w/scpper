const base = process.env.NEXT_PUBLIC_BASE_PATH ?? "/scpper";

const nextConfig = {
  reactStrictMode: true,
  output: "export",
  trailingSlash: true,
  basePath: base,
  images: { unoptimized: true },
  env: { NEXT_PUBLIC_BASE_PATH: base },
};

export default nextConfig;
