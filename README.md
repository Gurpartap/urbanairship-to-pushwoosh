## export UrbanAirship.com/devices | import PushWoosh.com/devices

Go based script for exporting device tokens from UrbanAirship and importing into PushWoosh push notifications service. Sadly PushWoosh doesn't have a bulk import API, so I made this script. Uses goroutines and channels for parallel export/import.

#### Note

This script is not very well tested. Tread carefully. Also, polling an API endpoint for every token is the worst way for bulk importing large number of device tokens. Ask out the PushWoosh guys to help with the bulk import if you have hundreds of thousands of tokens.

#### Usage

Enter your API keys and change some defaults in main.go file and run:

```bash
$ make
```

A JSON formatted dump of exported tokens will be available in the `./dump` directory.

#### Clean up dump

```bash
$ make clean
```

#### License

The MIT License (MIT)

Copyright (c) 2014 Gurpartap Singh

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
