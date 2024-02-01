const express = require('express');
const axios = require('axios');
const app = express();
const port = 3000;

var apm = require('elastic-apm-node').start({
// logLevel: 'none',
  serviceName: 'api-billing',
  secretToken: 'NotMyPassword',
  serverUrl: 'https://48ad4f8dab13weirdurl401c565c.apm.us-central1.gcp.cloud.es.io:443',
  environment: 'my-development'
})

app.use(express.json());

// Replace with the actual URL of your masking-api
const MASKING_API_URL = 'http://localhost:8080';

app.post('/get-user-details', async (req, res) => {
    const transaction = apm.startTransaction('/get-user-details', 'request');

    try {
        const { userId } = req.body;
        
        // Start a custom span for the external API call
        const span = transaction.startSpan('Calling MASKING_API', 'external.http');

        const response = await axios.post(`${MASKING_API_URL}/get-address`, { id: userId });

        // End the custom span after the API call is completed
        if (span) span.end();

        res.json(response.data);
    } catch (error) {
        console.error('Error calling masking-api:', error.message);
        apm.captureError(error); // Capture the error

        // End the custom span in case of an error
        if (span) span.end();

        res.status(500).send('Internal Server Error');
    } finally {
        transaction.end();
    }
});

app.listen(port, () => {
    console.log(`Billing API listening at http://localhost:${port}`);
});

