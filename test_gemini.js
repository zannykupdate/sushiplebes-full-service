const fs = require('fs');

async function testGemini() {
    const key = Object.fromEntries(fs.readFileSync('.env.example', 'utf-8').split('\n').filter(Boolean).map(x => x.split('='))).GEMINI_API_KEY.replace(/"/g, '');
    const url = `https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=${key}`;
    const reqData = {
        system_instruction: {
            parts: [{ text: "Respond in JSON" }]
        },
        contents: [
            {
                role: "user",
                parts: [{ text: "Hello" }]
            }
        ],
        generationConfig: {
            responseMimeType: "application/json",
            temperature: 0.2
        }
    };
    
    try {
        const fetch = globalThis.fetch || require('node-fetch');
        const res = await fetch(url, {
            method: 'POST',
            body: JSON.stringify(reqData),
            headers: { 'Content-Type': 'application/json' }
        });
        const text = await res.text();
        console.log("Status:", res.status);
        console.log("Response:", text);
    } catch(e) {
        console.error(e);
    }
}
testGemini();
