use serde::{Deserialize, Serialize};
use std::io::{self, Read};

#[derive(Deserialize)]
struct CompilerOptions {
    minify: Option<bool>,
    #[allow(dead_code)]
    beautify: Option<bool>,
    #[serde(rename = "keepComments")]
    keep_comments: Option<bool>,
}

#[derive(Deserialize)]
struct InputPayload {
    mjml: String,
    options: Option<CompilerOptions>,
}

#[derive(Serialize)]
struct ErrorDetail {
    message: String,
}

#[derive(Serialize)]
struct OutputPayload {
    html: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<ErrorDetail>,
}

fn print_error(msg: &str) {
    let out = OutputPayload {
        html: String::new(),
        error: Some(ErrorDetail {
            message: msg.to_string(),
        }),
    };
    if let Ok(json) = serde_json::to_string(&out) {
        println!("{}", json);
    }
}

fn main() {
    let mut buffer = String::new();
    if let Err(e) = io::stdin().read_to_string(&mut buffer) {
        print_error(&format!("failed to read stdin: {}", e));
        return;
    }

    let input: InputPayload = match serde_json::from_str(&buffer) {
        Ok(ip) => ip,
        Err(e) => {
            print_error(&format!("failed to parse JSON input: {}", e));
            return;
        }
    };

    // Set render options
    let mut render_opts = mrml::prelude::render::RenderOptions::default();
    let mut should_minify = false;

    if let Some(opts) = input.options {
        if let Some(keep_comments) = opts.keep_comments {
            render_opts.disable_comments = !keep_comments;
        }
        if let Some(minify) = opts.minify {
            should_minify = minify;
        }
    }

    // Parse MJML
    let parsed = match mrml::parse(&input.mjml) {
        Ok(p) => p,
        Err(e) => {
            print_error(&format!("mjml parse error: {:?}", e));
            return;
        }
    };

    // Render to HTML
    let mut html = match parsed.element.render(&render_opts) {
        Ok(h) => h,
        Err(e) => {
            print_error(&format!("mjml render error: {:?}", e));
            return;
        }
    };

    // Minify if requested
    if should_minify {
        let cfg = minify_html::Cfg::new();
        let minified = minify_html::minify(html.as_bytes(), &cfg);
        html = match String::from_utf8(minified) {
            Ok(s) => s,
            Err(e) => {
                print_error(&format!("minified HTML utf8 error: {}", e));
                return;
            }
        };
    }

    // Success response
    let out = OutputPayload {
        html,
        error: None,
    };

    match serde_json::to_string(&out) {
        Ok(json) => println!("{}", json),
        Err(e) => print_error(&format!("failed to serialize JSON output: {}", e)),
    }
}
