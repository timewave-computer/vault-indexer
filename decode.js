#!/usr/bin/env node

/**
 * Base64 Decoder
 * Usage: node base64-decoder.js <base64-string>
 */


function decodeBase64(base64String) {
    try {
        // Convert base64 to buffer
        const buffer = Buffer.from(base64String, 'base64');
        // Convert buffer to string
        return buffer.toString('utf-8');
    } catch (error) {
        console.error('Error decoding base64 string:', error.message);
        process.exit(1);
    }
}

// Get the base64 string from command line arguments
const base64String = process.argv[2];

if (!base64String) {
    console.log('Please provide a base64 string to decode');
    console.log('Usage: node base64-decoder.js <base64-string>');
    process.exit(1);
}

// Decode and print the result
const decodedString = decodeBase64(base64String);
console.log('Decoded string:', decodedString); 