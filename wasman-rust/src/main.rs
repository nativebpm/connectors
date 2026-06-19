use tonic::{transport::Server, Request, Response, Status};
use wasmtime::*;

pub mod runner {
    tonic::include_proto!("runner");
}

use runner::wasm_runner_server::{WasmRunner, WasmRunnerServer};
use runner::{WasmRequest, WasmResponse};

#[derive(Debug, Default)]
pub struct MyWasmRunner {}

#[tonic::async_trait]
impl WasmRunner for MyWasmRunner {
    async fn execute_wasm(
        &self,
        request: Request<WasmRequest>,
    ) -> Result<Response<WasmResponse>, Status> {
        let req = request.into_inner();
        println!("Executing WASM for instance: {}", req.instance_id);

        // 1. Initialize wasmtime Engine and Store
        let engine = Engine::default();
        let mut store = Store::new(&engine, ());

        // 2. Compile guest module from bytes
        let module = match Module::new(&engine, &req.wasm_bytes) {
            Ok(m) => m,
            Err(e) => return Ok(Response::new(WasmResponse {
                updated_variables: vec![],
                completed: false,
                error: format!("Failed to compile WASM: {}", e),
            })),
        };

        // 3. Define basic host imports (we'll expand this later with complete env & oplog bindings)
        let mut linker = Linker::new(&engine);
        
        // Mock proc_exit to intercept TinyGo exit code
        let linker_res = linker.func_wrap("wasi_snapshot_preview1", "proc_exit", |exit_code: i32| {
            println!("WASM guest exited with code: {}", exit_code);
        });

        if let Err(e) = linker_res {
            return Ok(Response::new(WasmResponse {
                updated_variables: vec![],
                completed: false,
                error: format!("Failed to bind host functions: {}", e),
            }));
        }

        // Instantiate the module
        let instance = match linker.instantiate(&mut store, &module) {
            Ok(i) => i,
            Err(e) => return Ok(Response::new(WasmResponse {
                updated_variables: vec![],
                completed: false,
                error: format!("Failed to instantiate module: {}", e),
            })),
        };

        // 4. Try calling the entrypoint function
        if let Ok(start_func) = instance.get_typed_func::<(), ()>(&mut store, "_start") {
            let _ = start_func.call(&mut store, ());
        }

        // Return mock variables for prototype verification
        Ok(Response::new(WasmResponse {
            updated_variables: req.initial_variables,
            completed: true,
            error: String::new(),
        }))
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let addr = "127.0.0.1:50051".parse()?;
    let runner = MyWasmRunner::default();

    println!("Wasman Rust Runner listening on {}", addr);

    Server::builder()
        .add_service(WasmRunnerServer::new(runner))
        .serve(addr)
        .await?;

    Ok(())
}
