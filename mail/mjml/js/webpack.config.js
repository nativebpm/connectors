const path = require("path");
const webpack = require("webpack");

module.exports = {
  mode: "production",
  target: "es2019",
  optimization: {
    sideEffects: true,
  },
  context: path.resolve(__dirname, "src"),
  entry: "./index.js",
  resolve: {
    fallback: {
      fs: false,
      https: false,
      http: false,
      os: false,
      path: false,
      url: false,
    },
    alias: {
      "uglify-js": path.resolve(__dirname, "shims/uglify-js"), // We need to do this because uglify-js can't be built with webpack
    },
  },
  output: {
    path: path.resolve(__dirname, "dist"),
    filename: "mjml.js",
    chunkFormat: "commonjs",
  },
  plugins: [
    new webpack.ProvidePlugin({
      process: "process/browser",
      window: "global/window",
    }),
  ],
  performance: {
    hints: false,
  },
};
