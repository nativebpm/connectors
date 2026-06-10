import { compile } from './lib';

const decoder = new TextDecoder();

function readAllStdin() {
  const chunks = [];
  let totalLength = 0;
  const chunkSize = 4096;

  while (true) {
    const buffer = new Uint8Array(chunkSize);
    const bytesRead = Javy.IO.readSync(0, buffer);
    if (bytesRead <= 0) {
      break;
    }
    if (bytesRead === chunkSize) {
      chunks.push(buffer);
    } else {
      chunks.push(buffer.subarray(0, bytesRead));
    }
    totalLength += bytesRead;
  }

  const result = new Uint8Array(totalLength);
  let offset = 0;
  for (const chunk of chunks) {
    result.set(chunk, offset);
    offset += chunk.length;
  }

  return result;
}

function main() {
  let inputBytes;
  try {
    inputBytes = readAllStdin();
  } catch (err) {
    const errorBytes = new TextEncoder().encode(JSON.stringify({
      error: {
        message: "failed to read from stdin: " + err.message
      }
    }));
    Javy.IO.writeSync(1, errorBytes);
    return;
  }

  const input = decoder.decode(inputBytes);

  let encodedJSON;

  try {
    const decodedJSON = JSON.parse(input);
    const result = compile(decodedJSON);
    encodedJSON = JSON.stringify(result);
  } catch (err) {
    encodedJSON = JSON.stringify({
      error: {
        message: err.message,
      },
    });
  }

  const outputBytes = new TextEncoder().encode(encodedJSON);
  Javy.IO.writeSync(1, outputBytes);
}

main();
