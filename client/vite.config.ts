import { defineConfig } from "vite";

// 開発サーバー設定。
// /api はバックエンド(Go, :8080)へプロキシ＝同一オリジン化（ブラウザ↔WSLのlocalhost差異・CORS回避）。
// WSL の /mnt 上では inotify が効かないため監視はポーリング。
export default defineConfig({
  server: {
    watch: { usePolling: true, interval: 300 },
    proxy: {
      "/api": "http://localhost:8080",
    },
  },
});
